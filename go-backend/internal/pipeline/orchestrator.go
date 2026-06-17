package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/parser"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/similarity"
	"github.com/smartedu/training-eval-system/internal/sse"
	"github.com/smartedu/training-eval-system/internal/worker"
)

// OrchestratorDeps groups constructor dependencies.
type OrchestratorDeps struct {
	Pool          *worker.Pool
	Broker        *sse.Broker
	UploadRepo    repository.UploadRepo
	EvalRepo      repository.EvaluationRepo
	SimRepo       repository.SimilarityRepo
	TaskRepo      repository.TaskRepo
	ProfileRepo   repository.ProfileRepo
	SystemCfgRepo repository.SystemConfigRepo
	LLMClient     *llm.Client
	OnScored      func(studentID int64) // called after scoring completes to trigger profile recompute
}

// Orchestrator coordinates the multi-stage evaluation pipeline.
type Orchestrator struct {
	pool        *worker.Pool
	broker      *sse.Broker
	uploadRepo  repository.UploadRepo
	evalRepo    repository.EvaluationRepo
	simRepo     repository.SimilarityRepo
	taskRepo    repository.TaskRepo
	profileRepo repository.ProfileRepo
	llmClient   *llm.Client
	onScored    func(studentID int64)

	scorer     *Scorer
	verifier   *Verifier
	simChecker *SimilarityChecker

	// In-memory tracking of active scoring tasks
	mu          sync.RWMutex
	activeTasks map[int64]struct{} // uploadID -> active scoring
}

// NewOrchestrator creates a pipeline orchestrator with all dependencies injected.
func NewOrchestrator(deps OrchestratorDeps) *Orchestrator {
	o := &Orchestrator{
		pool:        deps.Pool,
		broker:      deps.Broker,
		uploadRepo:  deps.UploadRepo,
		evalRepo:    deps.EvalRepo,
		simRepo:     deps.SimRepo,
		taskRepo:    deps.TaskRepo,
		profileRepo: deps.ProfileRepo,
		llmClient:   deps.LLMClient,
		onScored:    deps.OnScored,
		activeTasks: make(map[int64]struct{}),
	}

	o.scorer = &Scorer{
		client:   deps.LLMClient,
		evalRepo: deps.EvalRepo,
		taskRepo: deps.TaskRepo,
		broker:   deps.Broker,
	}
	if deps.SystemCfgRepo != nil {
		o.scorer.SetSystemConfigRepo(deps.SystemCfgRepo)
	}
	o.verifier = &Verifier{
		client:     deps.LLMClient,
		uploadRepo: deps.UploadRepo,
		taskRepo:   deps.TaskRepo,
	}
	o.verifier.SetBroker(deps.Broker)
	o.simChecker = &SimilarityChecker{
		uploadRepo: deps.UploadRepo,
		simRepo:    deps.SimRepo,
		taskRepo:   deps.TaskRepo,
		broker:     deps.Broker,
	}

	return o
}

// TriggerParse initiates the parse stage for a pending upload.
func (o *Orchestrator) TriggerParse(ctx context.Context, uploadID int64) error {
	upload, err := o.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("pipeline: upload not found: %w", err)
	}

	// Update status to parsing
	if err := o.uploadRepo.UpdateStatus(ctx, uploadID, "parsing"); err != nil {
		return fmt.Errorf("pipeline: update status: %w", err)
	}

	// Publish SSE
	o.publishParseProgress(upload.StudentID, uploadID, "parsing", "")

	// Submit parse task to worker pool
	task := &worker.Task{
		ID: fmt.Sprintf("parse-%d", uploadID),
		Fn: func(taskCtx context.Context) error {
			return o.executeParse(taskCtx, upload)
		},
	}

	if err := o.pool.Submit(task); err != nil {
		// Revert status on queue full
		_ = o.uploadRepo.UpdateStatus(ctx, uploadID, "pending")
		return fmt.Errorf("pipeline: submit parse task: %w", err)
	}

	return nil
}

// TriggerRetry re-submits a failed upload for re-parsing.
func (o *Orchestrator) TriggerRetry(ctx context.Context, uploadID int64) error {
	upload, err := o.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("pipeline: upload not found: %w", err)
	}

	if upload.ParseStatus != "failed" {
		return fmt.Errorf("pipeline: can only retry failed uploads, current status: %s", upload.ParseStatus)
	}

	// Reset to pending then trigger parse
	if err := o.uploadRepo.UpdateStatus(ctx, uploadID, "pending"); err != nil {
		return fmt.Errorf("pipeline: reset status: %w", err)
	}

	return o.TriggerParse(ctx, uploadID)
}

// IsScoringActive returns whether a scoring task is in progress for the upload.
func (o *Orchestrator) IsScoringActive(uploadID int64) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	_, ok := o.activeTasks[uploadID]
	return ok
}

func (o *Orchestrator) markScoringActive(uploadID int64) {
	o.mu.Lock()
	o.activeTasks[uploadID] = struct{}{}
	o.mu.Unlock()
}

func (o *Orchestrator) markScoringDone(uploadID int64) {
	o.mu.Lock()
	delete(o.activeTasks, uploadID)
	o.mu.Unlock()
}

// executeParse runs the document parsing logic.
func (o *Orchestrator) executeParse(ctx context.Context, upload *model.Upload) error {
	var rawText string
	var parseErr error

	switch upload.FileType {
	case "docx":
		rawText, parseErr = o.parseDocx(upload.StoragePath)
	case "doc":
		// .doc files: try docx parser first (many .doc files are actually docx format)
		rawText, parseErr = o.parseDocx(upload.StoragePath)
		if parseErr != nil {
			// Try the legacy .doc parser (extracts UTF-16LE text from OLE2 binary)
			rawText, parseErr = o.parseDocWithOCR(ctx, upload.StoragePath)
		} else {
			// Even if docx parser succeeded, also try to extract images for OCR
			imgText, _ := o.ocrDocImages(ctx, upload.StoragePath)
			if imgText != "" {
				rawText = rawText + "\n\n--- 图片内容 ---\n" + imgText
			}
		}
	case "pdf":
		rawText, parseErr = o.parsePDF(upload.StoragePath)
	case "png", "jpg", "jpeg":
		rawText, parseErr = o.parseImage(ctx, upload.StoragePath, upload.FileType)
	default:
		parseErr = fmt.Errorf("unsupported file type: %s", upload.FileType)
	}

	if parseErr != nil {
		// Save failure
		slog.Error("parse failed", "upload_id", upload.ID, "error", parseErr.Error())
		_ = o.uploadRepo.UpdateStatus(ctx, upload.ID, "failed")
		_ = o.uploadRepo.SaveParseResult(ctx, &model.ParseResult{
			UploadID:     upload.ID,
			ErrorMessage: parseErr.Error(),
		})
		o.publishParseProgress(upload.StudentID, upload.ID, "failed", parseErr.Error())
		return parseErr
	}

	if rawText == "" {
		parseErr = fmt.Errorf("extracted text is empty")
		_ = o.uploadRepo.UpdateStatus(ctx, upload.ID, "failed")
		_ = o.uploadRepo.SaveParseResult(ctx, &model.ParseResult{
			UploadID:     upload.ID,
			ErrorMessage: parseErr.Error(),
		})
		o.publishParseProgress(upload.StudentID, upload.ID, "failed", parseErr.Error())
		return parseErr
	}

	// Compute simhash
	simhash := similarity.SimHash(rawText)

	// Save parse result
	simhashVal := int64(simhash)
	pr := &model.ParseResult{
		UploadID: upload.ID,
		RawText:  rawText,
		SimHash:  &simhashVal,
	}
	if err := o.uploadRepo.SaveParseResult(ctx, pr); err != nil {
		_ = o.uploadRepo.UpdateStatus(ctx, upload.ID, "failed")
		o.publishParseProgress(upload.StudentID, upload.ID, "failed", err.Error())
		return err
	}

	// Mark as parsed
	_ = o.uploadRepo.UpdateStatus(ctx, upload.ID, "parsed")
	o.publishParseProgress(upload.StudentID, upload.ID, "parsed", "")

	// Trigger downstream stages
	o.onParseComplete(ctx, upload, rawText, simhash)

	return nil
}

// onParseComplete triggers verify, score, and similarity stages in parallel.
func (o *Orchestrator) onParseComplete(ctx context.Context, upload *model.Upload, rawText string, simhash uint64) {
	// 1. Submit verification task
	_ = o.pool.Submit(&worker.Task{
		ID: fmt.Sprintf("verify-%d", upload.ID),
		Fn: func(taskCtx context.Context) error {
			return o.verifier.Verify(taskCtx, upload.ID, rawText)
		},
	})

	// 2. Create evaluation and submit scoring task
	eval := &model.Evaluation{
		TaskID:    upload.TaskID,
		StudentID: upload.StudentID,
		UploadID:  upload.ID,
		Status:    "pending",
	}
	if err := o.evalRepo.Create(ctx, eval); err != nil {
		slog.Error("pipeline: create evaluation", "error", err.Error())
		return
	}

	o.markScoringActive(upload.ID)
	o.publishEvalProgress(upload.StudentID, upload.ID, "scoring")

	_ = o.pool.Submit(&worker.Task{
		ID: fmt.Sprintf("score-%d", eval.ID),
		Fn: func(taskCtx context.Context) error {
			defer o.markScoringDone(upload.ID)
			if err := o.scorer.Score(taskCtx, eval.ID, rawText); err != nil {
				return err
			}
			// Trigger profile recompute for this student
			if o.onScored != nil {
				o.onScored(upload.StudentID)
			}
			return nil
		},
	})

	// 3. Submit similarity check task
	_ = o.pool.Submit(&worker.Task{
		ID: fmt.Sprintf("similarity-%d", upload.ID),
		Fn: func(taskCtx context.Context) error {
			return o.simChecker.Check(taskCtx, upload.ID, upload.TaskID, int64(simhash))
		},
	})
}

// RecoverStuck scans for uploads that are parsed but have no scored evaluation,
// and re-triggers the scoring pipeline for them. Also handles uploads stuck in
// "parsing" status (server crashed during parse) by resetting them to "pending"
// and re-triggering parse.
func (o *Orchestrator) RecoverStuck(ctx context.Context) {
	slog.Info("pipeline recovery: starting stuck task scan")

	// --- 1. Handle uploads stuck in "parsing" status ---
	parsingStatus := "parsing"
	parsingUploads, _, err := o.uploadRepo.List(ctx, repository.UploadListParams{
		ParseStatus: &parsingStatus,
		ListParams:  repository.ListParams{Page: 1, PageSize: 1000},
	})
	if err != nil {
		slog.Error("pipeline recovery: failed to list parsing uploads", "error", err)
	} else {
		for _, u := range parsingUploads {
			slog.Info("pipeline recovery: resetting stuck parsing upload", "upload_id", u.ID)
			if err := o.uploadRepo.UpdateStatus(ctx, u.ID, "pending"); err != nil {
				slog.Error("pipeline recovery: failed to reset parsing upload", "upload_id", u.ID, "error", err)
				continue
			}
			if err := o.TriggerParse(ctx, u.ID); err != nil {
				slog.Error("pipeline recovery: failed to re-trigger parse", "upload_id", u.ID, "error", err)
			}
		}
	}

	// --- 2. Handle uploads in "parsed" status with no scored evaluation ---
	parsedStatus := "parsed"
	parsedUploads, _, err := o.uploadRepo.List(ctx, repository.UploadListParams{
		ParseStatus: &parsedStatus,
		ListParams:  repository.ListParams{Page: 1, PageSize: 1000},
	})
	if err != nil {
		slog.Error("pipeline recovery: failed to list parsed uploads", "error", err)
		return
	}

	recovered := 0
	for _, u := range parsedUploads {
		// Check for evaluations with scored or confirmed status
		scoredStatus := "scored"
		scoredEvals, _, _ := o.evalRepo.List(ctx, repository.EvalListParams{
			UploadID:   &u.ID,
			Status:     &scoredStatus,
			ListParams: repository.ListParams{Page: 1, PageSize: 1},
		})
		confirmedStatus := "confirmed"
		confirmedEvals, _, _ := o.evalRepo.List(ctx, repository.EvalListParams{
			UploadID:   &u.ID,
			Status:     &confirmedStatus,
			ListParams: repository.ListParams{Page: 1, PageSize: 1},
		})

		if len(scoredEvals) > 0 || len(confirmedEvals) > 0 {
			continue // has a completed evaluation, skip
		}

		// Check for pending evaluations (created but scoring never ran)
		pendingStatus := "pending"
		pendingEvals, _, _ := o.evalRepo.List(ctx, repository.EvalListParams{
			UploadID:   &u.ID,
			Status:     &pendingStatus,
			ListParams: repository.ListParams{Page: 1, PageSize: 100},
		})

		for _, ev := range pendingEvals {
			if ev.OverallComment == "" {
				// Pending with no overall_comment: scoring never ran, delete it
				slog.Info("pipeline recovery: deleting stuck pending evaluation", "eval_id", ev.ID, "upload_id", u.ID)
				if err := o.evalRepo.Delete(ctx, ev.ID); err != nil {
					slog.Error("pipeline recovery: failed to delete stuck evaluation", "eval_id", ev.ID, "error", err)
				}
			}
		}

		// Re-trigger onParseComplete
		pr, err := o.uploadRepo.GetParseResult(ctx, u.ID)
		if err != nil || pr == nil {
			slog.Error("pipeline recovery: no parse result for parsed upload", "upload_id", u.ID, "error", err)
			continue
		}
		if pr.RawText == "" {
			slog.Error("pipeline recovery: empty raw text for parsed upload", "upload_id", u.ID)
			continue
		}

		var simhash uint64
		if pr.SimHash != nil {
			simhash = uint64(*pr.SimHash)
		}

		slog.Info("pipeline recovery: re-triggering scoring for parsed upload", "upload_id", u.ID)
		o.onParseComplete(ctx, &u, pr.RawText, simhash)
		recovered++
	}

	slog.Info("pipeline recovery: scan complete", "parsed_checked", len(parsedUploads), "re_triggered", recovered)
}

// --- File parsing helpers ---

func (o *Orchestrator) parseDocx(storagePath string) (string, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read docx file: %w", err)
	}
	return parser.ParseDocxBytes(data)
}

func (o *Orchestrator) parseDoc(storagePath string) (string, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read doc file: %w", err)
	}
	return parser.ParseDoc(data)
}

// parseDocWithOCR extracts text from .doc and also OCRs embedded images via multimodal LLM.
func (o *Orchestrator) parseDocWithOCR(ctx context.Context, storagePath string) (string, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read doc file: %w", err)
	}

	// Extract text from OLE2 binary
	text, err := parser.ParseDoc(data)
	if err != nil {
		return "", err
	}

	// Extract and OCR embedded images
	imgText, imgErr := o.ocrDocImagesFromBytes(ctx, data)
	if imgErr != nil {
		slog.Warn("parseDocWithOCR: image OCR failed, continuing with text only", "error", imgErr.Error())
	}
	if imgText != "" {
		text = text + "\n\n--- 图片内容 ---\n" + imgText
	}

	return text, nil
}

// ocrDocImages extracts images from a .doc file and OCRs them via multimodal LLM.
func (o *Orchestrator) ocrDocImages(ctx context.Context, storagePath string) (string, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read doc file for image extraction: %w", err)
	}
	return o.ocrDocImagesFromBytes(ctx, data)
}

// ocrDocImagesFromBytes extracts images from .doc binary data and OCRs them.
func (o *Orchestrator) ocrDocImagesFromBytes(ctx context.Context, data []byte) (string, error) {
	if o.llmClient == nil {
		return "", fmt.Errorf("LLM client not configured for OCR")
	}

	images := parser.ExtractDocImages(data)
	if len(images) == 0 {
		return "", nil
	}

	slog.Info("parseDoc: extracted images from .doc", "count", len(images))

	var allText strings.Builder
	for idx, imgData := range images {
		// Skip very small images (likely icons or artifacts, < 1KB)
		if len(imgData) < 1024 {
			continue
		}

		// Rate limit: wait between images to avoid API 429 errors
		if idx > 0 {
			select {
			case <-time.After(1 * time.Second):
			case <-ctx.Done():
				return allText.String(), ctx.Err()
			}
		}

		b64 := base64.StdEncoding.EncodeToString(imgData)
		mimeType := "image/png"
		// Check if JPEG
		if len(imgData) > 2 && imgData[0] == 0xFF && imgData[1] == 0xD8 {
			mimeType = "image/jpeg"
		}

		ocrText, err := o.llmClient.ExtractTextFromImage(ctx, b64, mimeType)
		if err != nil {
			slog.Warn("parseDoc: OCR failed for image", "index", idx, "error", err.Error())
			continue
		}

		if ocrText != "" {
			allText.WriteString(fmt.Sprintf("\n[图片%d 内容]\n%s\n", idx+1, ocrText))
		}
	}

	return allText.String(), nil
}

func (o *Orchestrator) parsePDF(storagePath string) (string, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read pdf file: %w", err)
	}
	return parser.ParsePDFBytes(data)
}

func (o *Orchestrator) parseImage(ctx context.Context, storagePath string, fileType string) (string, error) {
	if o.llmClient == nil {
		return "", fmt.Errorf("LLM client not configured for OCR")
	}

	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read image file: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	mimeType := "image/" + fileType
	if fileType == "jpg" {
		mimeType = "image/jpeg"
	}

	return o.llmClient.ExtractTextFromImage(ctx, b64, mimeType)
}

// --- SSE helpers ---

func (o *Orchestrator) publishParseProgress(userID, uploadID int64, status, errMsg string) {
	payload := map[string]any{"upload_id": uploadID, "status": status, "stage": "parse"}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	data, _ := json.Marshal(payload)
	o.broker.Publish(sse.Event{
		UserID: userID,
		Type:   "progress",
		Data:   string(data),
	})
}

func (o *Orchestrator) publishEvalProgress(userID, uploadID int64, status string) {
	data, _ := json.Marshal(map[string]any{"upload_id": uploadID, "status": status, "stage": "eval"})
	o.broker.Publish(sse.Event{
		UserID: userID,
		Type:   "progress",
		Data:   string(data),
	})
}
