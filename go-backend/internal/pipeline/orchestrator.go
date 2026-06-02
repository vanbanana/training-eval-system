package pipeline

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

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
	Pool        *worker.Pool
	Broker      *sse.Broker
	UploadRepo  repository.UploadRepo
	EvalRepo    repository.EvaluationRepo
	SimRepo     repository.SimilarityRepo
	TaskRepo    repository.TaskRepo
	ProfileRepo repository.ProfileRepo
	LLMClient   *llm.Client
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
		activeTasks: make(map[int64]struct{}),
	}

	o.scorer = &Scorer{
		client:   deps.LLMClient,
		evalRepo: deps.EvalRepo,
		taskRepo: deps.TaskRepo,
		broker:   deps.Broker,
	}
	o.verifier = &Verifier{
		client:     deps.LLMClient,
		uploadRepo: deps.UploadRepo,
		taskRepo:   deps.TaskRepo,
	}
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
	case "docx", "doc":
		rawText, parseErr = o.parseDocx(upload.StoragePath)
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
			return o.scorer.Score(taskCtx, eval.ID, rawText)
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

// --- File parsing helpers ---

func (o *Orchestrator) parseDocx(storagePath string) (string, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return "", fmt.Errorf("read docx file: %w", err)
	}
	return parser.ParseDocxBytes(data)
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
	payload := map[string]any{"upload_id": uploadID, "status": status}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	data, _ := json.Marshal(payload)
	o.broker.Publish(sse.Event{
		UserID: userID,
		Type:   "parse_progress",
		Data:   string(data),
	})
}

func (o *Orchestrator) publishEvalProgress(userID, uploadID int64, status string) {
	data, _ := json.Marshal(map[string]any{"upload_id": uploadID, "status": status})
	o.broker.Publish(sse.Event{
		UserID: userID,
		Type:   "eval_progress",
		Data:   string(data),
	})
}
