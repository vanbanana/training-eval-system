package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/sse"
)

// Verifier handles LLM-based requirement verification.
type Verifier struct {
	client     *llm.Client
	uploadRepo repository.UploadRepo
	taskRepo   repository.TaskRepo
	broker     *sse.Broker
}

// SetBroker sets the SSE broker for publishing verification results/failures.
func (v *Verifier) SetBroker(broker *sse.Broker) {
	v.broker = broker
}

// Verify executes requirement verification for a parsed upload.
// On LLM failure, marks the upload as verify_failed and notifies via SSE.
func (v *Verifier) Verify(ctx context.Context, uploadID int64, rawText string) error {
	upload, err := v.uploadRepo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("verifier: load upload: %w", err)
	}

	task, err := v.taskRepo.GetByID(ctx, upload.TaskID)
	if err != nil {
		return fmt.Errorf("verifier: load task: %w", err)
	}

	if task.Requirements == "" {
		slog.Info("verifier: task has no requirements, skipping", "task_id", task.ID)
		return nil
	}

	// Build prompt and tool schema
	messages := llm.BuildVerificationPrompt(task.Requirements, rawText)
	tool := llm.VerificationToolSchema()

	// Attempt verification with retries
	var verifyResult *VerifyToolResponse
	maxAttempts := 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := v.client.Complete(ctx, messages, []llm.Tool{tool})
		if err != nil {
			slog.Error("verifier: LLM call failed", "upload_id", uploadID, "attempt", attempt+1, "error", err.Error())
			if attempt == maxAttempts-1 {
				return v.markVerifyFailed(ctx, upload, task, fmt.Sprintf("LLM unavailable after %d attempts", maxAttempts))
			}
			continue
		}

		verifyResult, err = parseVerifyToolCall(resp)
		if err != nil {
			slog.Warn("verifier: parse response failed", "upload_id", uploadID, "attempt", attempt+1, "error", err.Error())
			if attempt == maxAttempts-1 {
				return v.markVerifyFailed(ctx, upload, task, fmt.Sprintf("parse failure after %d attempts: %v", maxAttempts, err))
			}
			continue
		}
		break
	}

	if verifyResult == nil {
		return v.markVerifyFailed(ctx, upload, task, "all verification attempts returned nil result")
	}

	// Save verify result
	matchRate := verifyResult.MatchRate
	confidence := int(verifyResult.MatchRate)
	vr := &model.VerifyResult{
		UploadID:          uploadID,
		MatchRate:         &matchRate,
		Checkpoints:       verifyResult.Checkpoints,
		MissingItems:      verifyResult.MissingItems,
		LogicIssues:       verifyResult.LogicIssues,
		OverallConfidence: &confidence,
	}

	if err := v.uploadRepo.SaveVerifyResult(ctx, vr); err != nil {
		slog.Error("verifier: save result failed", "upload_id", uploadID, "error", err.Error())
		return nil
	}

	slog.Info("verifier: verification complete", "upload_id", uploadID, "match_rate", verifyResult.MatchRate)

	// Publish SSE success event
	if v.broker != nil {
		v.broker.Publish(sse.Event{
			UserID: task.TeacherID,
			Type:   "verify_complete",
			Data:   fmt.Sprintf(`{"upload_id":%d,"match_rate":%.0f}`, uploadID, matchRate),
		})
	}

	return nil
}

// markVerifyFailed updates the upload state and notifies the teacher.
func (v *Verifier) markVerifyFailed(ctx context.Context, upload *model.Upload, task *model.TrainingTask, reason string) error {
	slog.Error("verifier: marking upload as failed (verification failed)", "upload_id", upload.ID, "reason", reason)

	// parse_status only accepts pending/parsing/parsed/failed; verification is a
	// post-parse step, so a verification failure is recorded as "failed".
	_ = v.uploadRepo.UpdateStatus(ctx, upload.ID, "failed")
	_ = v.uploadRepo.SaveVerifyResult(ctx, &model.VerifyResult{
		UploadID:     upload.ID,
		ErrorMessage: reason,
	})

	// Notify teacher via SSE (event name is independent of the DB status value).
	if v.broker != nil && task != nil {
		v.broker.Publish(sse.Event{
			UserID: task.TeacherID,
			Type:   "verify_failed",
			Data:   fmt.Sprintf(`{"upload_id":%d,"task_id":%d,"reason":%q}`, upload.ID, task.ID, reason),
		})
	}

	return fmt.Errorf("verifier: verification failed for upload %d: %s", upload.ID, reason)
}
