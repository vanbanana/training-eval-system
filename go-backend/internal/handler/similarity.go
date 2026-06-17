package handler

import (
	"net/http"
	"sort"
	"strings"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/middleware"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/similarity"
)

type SimilarityHandler struct {
	simRepo    repository.SimilarityRepo
	uploadRepo repository.UploadRepo
}

func NewSimilarityHandler(simRepo repository.SimilarityRepo, uploadRepo repository.UploadRepo) *SimilarityHandler {
	return &SimilarityHandler{simRepo: simRepo, uploadRepo: uploadRepo}
}

func (h *SimilarityHandler) GetByTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := PathInt64(r, "taskId")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	var state *string
	if s := QueryStr(r, "state", ""); s != "" {
		state = &s
	}

	records, err := h.simRepo.List(r.Context(), taskID, state)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]dto.SimilarityRecordResponse, 0, len(records))
	for _, rec := range records {
		resp := dto.SimilarityRecordResponse{
			ID: rec.ID, TaskID: rec.TaskID,
			UploadAID: rec.UploadAID, UploadBID: rec.UploadBID,
			HammingDistance: rec.HammingDistance, CosineSimilarity: rec.CosineSimilarity,
			State: rec.State, ReviewedBy: rec.ReviewedBy,
			CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if rec.DecidedAt != nil {
			s := rec.DecidedAt.Format("2006-01-02T15:04:05Z07:00")
			resp.DecidedAt = &s
		}
		items = append(items, resp)
	}
	JSON(w, http.StatusOK, items)
}

func (h *SimilarityHandler) GetSegments(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid record ID")
		return
	}
	ctx := r.Context()

	record, err := h.simRepo.GetByID(ctx, id)
	if err != nil || record == nil {
		Error(w, http.StatusNotFound, "Similarity record not found")
		return
	}

	prA, errA := h.uploadRepo.GetParseResult(ctx, record.UploadAID)
	prB, errB := h.uploadRepo.GetParseResult(ctx, record.UploadBID)
	if errA != nil || errB != nil || prA == nil || prB == nil {
		// Parse results unavailable; return empty segment list rather than failing.
		JSON(w, http.StatusOK, []dto.SegmentPairResponse{})
		return
	}

	pairs := computeSegmentPairs(prA.RawText, prB.RawText)
	JSON(w, http.StatusOK, pairs)
}

func (h *SimilarityHandler) UpdateDecision(w http.ResponseWriter, r *http.Request) {
	id, err := PathInt64(r, "id")
	if err != nil {
		Error(w, http.StatusBadRequest, "Invalid record ID")
		return
	}

	var req dto.SimilarityDecisionRequest
	if err := Decode(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Map action to a valid DB state (CHECK: suspect/confirmed/ignored).
	state := req.Action
	switch req.Action {
	case "confirm", "confirmed":
		state = "confirmed"
	case "ignore", "ignored", "dismiss":
		state = "ignored"
	default:
		Error(w, http.StatusBadRequest, "Invalid action (expected confirm or ignore)")
		return
	}

	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if _, err := h.simRepo.GetByID(r.Context(), id); err != nil {
		Error(w, http.StatusNotFound, "Similarity record not found")
		return
	}

	if err := h.simRepo.UpdateState(r.Context(), id, state, claims.Sub); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Decision recorded"})
}

// computeSegmentPairs finds the most similar sentence pairs between two texts
// using SimHash hamming distance at the sentence level.
// Returns pairs with byte-offset positions in the original texts.
func computeSegmentPairs(textA, textB string) []dto.SegmentPairResponse {
	segsA := splitSentencesWithPos(textA)
	segsB := splitSentencesWithPos(textB)

	const maxPairs = 20
	var pairs []dto.SegmentPairResponse
	for _, a := range segsA {
		ha := similarity.SimHash(a.text)
		bestSim := 0.0
		var bestB sentenceInfo
		for _, b := range segsB {
			hb := similarity.SimHash(b.text)
			dist := similarity.HammingDistance(ha, hb)
			sim := 1.0 - float64(dist)/64.0
			if sim > bestSim {
				bestSim = sim
				bestB = b
			}
		}
		if bestSim >= 0.75 && bestB.text != "" {
			pairs = append(pairs, dto.SegmentPairResponse{
				AStart:   a.start,
				AEnd:     a.end,
				BStart:   bestB.start,
				BEnd:     bestB.end,
				SnippetA: a.text,
				SnippetB: bestB.text,
				Ratio:    bestSim,
			})
		}
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Ratio > pairs[j].Ratio })
	if len(pairs) > maxPairs {
		pairs = pairs[:maxPairs]
	}
	if pairs == nil {
		pairs = []dto.SegmentPairResponse{}
	}
	return pairs
}

type sentenceInfo struct {
	text  string
	start int
	end   int
}

// splitSentencesWithPos splits text into non-trivial sentences on common CJK/ASCII delimiters,
// returning each sentence along with its byte-offset range in the original text.
func splitSentencesWithPos(text string) []sentenceInfo {
	delimiters := func(rc rune) bool {
		switch rc {
		case '。', '！', '？', '\n', '.', '!', '?', ';', '；':
			return true
		}
		return false
	}

	var out []sentenceInfo
	pos := 0
	for pos < len(text) {
		// Skip leading delimiters
		for pos < len(text) && delimiters(rune(text[pos])) {
			pos++
		}
		if pos >= len(text) {
			break
		}
		start := pos
		// Find next delimiter
		for pos < len(text) && !delimiters(rune(text[pos])) {
			pos++
		}
		seg := strings.TrimSpace(text[start:pos])
		if len([]rune(seg)) >= 8 { // ignore very short fragments
			out = append(out, sentenceInfo{text: seg, start: start, end: pos})
		}
	}
	return out
}
