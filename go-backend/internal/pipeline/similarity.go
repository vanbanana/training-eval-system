package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/similarity"
	"github.com/smartedu/training-eval-system/internal/sse"
)

// SimilarityChecker compares new parse results against existing ones using
// a two-phase approach: SimHash coarse filtering → embedding cosine similarity.
type SimilarityChecker struct {
	uploadRepo repository.UploadRepo
	simRepo    repository.SimilarityRepo
	taskRepo   repository.TaskRepo
	broker     *sse.Broker
}

const hammingThreshold = 3

// HammingThreshold is the max hamming distance for similarity suspect (exported for tests).
const HammingThreshold = hammingThreshold
const cosineThresholdFlag = 0.85

// OrderPair ensures a < b for the ordering invariant (exported for tests).
func OrderPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

// Check compares the given upload's content against all others for the same task.
// Phase 1: SimHash coarse filtering (Hamming distance < threshold)
// Phase 2: For candidates passing Phase 1, compute local embedding cosine similarity (requirement 18.3)
func (sc *SimilarityChecker) Check(ctx context.Context, uploadID int64, taskID int64, simhash int64) error {
	allRecords, err := sc.simRepo.List(ctx, taskID, nil)
	if err != nil {
		slog.Warn("similarity: list existing records failed", "error", err.Error())
		allRecords = nil
	}

	existingPairs := make(map[[2]int64]bool)
	for _, r := range allRecords {
		existingPairs[[2]int64{r.UploadAID, r.UploadBID}] = true
	}

	params := repository.UploadListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
	}
	params.TaskID = &taskID
	uploads, _, err := sc.uploadRepo.List(ctx, params)
	if err != nil {
		return fmt.Errorf("similarity: list uploads: %w", err)
	}

	task, _ := sc.taskRepo.GetByID(ctx, taskID)
	var teacherID int64
	if task != nil {
		teacherID = task.TeacherID
	}

	ownPR, _ := sc.uploadRepo.GetParseResult(ctx, uploadID)
	var ownEmbedding []float64
	if ownPR != nil && len(ownPR.Embedding) > 0 {
		ownEmbedding = ownPR.Embedding
	} else if ownPR != nil && ownPR.RawText != "" {
		ownEmbedding = similarity.ComputeLocalEmbedding(ownPR.RawText)
	}

	for _, other := range uploads {
		if other.ID == uploadID {
			continue
		}

		pr, err := sc.uploadRepo.GetParseResult(ctx, other.ID)
		if err != nil || pr == nil || pr.SimHash == nil {
			continue
		}

		aID, bID := OrderPair(uploadID, other.ID)
		if existingPairs[[2]int64{aID, bID}] {
			continue
		}

		dist := similarity.HammingDistance(uint64(simhash), uint64(*pr.SimHash))
		if dist >= hammingThreshold {
			continue
		}

		var cosineSim float64
		otherEmbedding := pr.Embedding
		if len(otherEmbedding) == 0 && pr.RawText != "" {
			otherEmbedding = similarity.ComputeLocalEmbedding(pr.RawText)
		}

		if len(ownEmbedding) > 0 && len(otherEmbedding) > 0 {
			cosineSim = similarity.CosineSimilarity(ownEmbedding, otherEmbedding)
		} else {
			cosineSim = 1.0 - float64(dist)/64.0
		}

		// Hamming distance flagged this pair as a candidate. Use cosine similarity
		// as a second gate: high cosine => genuine suspect; low cosine => treat as
		// a non-match and record it as "ignored" (a valid state) so it is retained
		// for audit but never raises an alert.
		state := "suspect"
		if cosineSim < cosineThresholdFlag {
			state = "ignored"
		}

		record := &model.SimilarityRecord{
			TaskID:           taskID,
			UploadAID:        aID,
			UploadBID:        bID,
			HammingDistance:  dist,
			CosineSimilarity: &cosineSim,
			State:            state,
		}

		if err := sc.simRepo.Create(ctx, record); err != nil {
			slog.Warn("similarity: create record failed", "a", aID, "b", bID, "error", err.Error())
			continue
		}

		existingPairs[[2]int64{aID, bID}] = true

		if state == "suspect" && teacherID > 0 {
			sc.broker.Publish(sse.Event{
				UserID: teacherID,
				Type:   "similarity_alert",
				Data: fmt.Sprintf(`{"task_id":%d,"upload_a_id":%d,"upload_b_id":%d,"hamming_distance":%d,"cosine_similarity":%.2f,"state":"%s"}`,
					taskID, aID, bID, dist, cosineSim, state),
			})
		}

		slog.Info("similarity: pair analyzed",
			"task_id", taskID, "upload_a", aID, "upload_b", bID,
			"hamming", dist, "cosine", fmt.Sprintf("%.2f", cosineSim), "state", state)
	}

	return nil
}
