package handler

import (
	"net/http"

	"github.com/smartedu/training-eval-system/internal/dto"
)

type SimilarityHandler struct{}

func NewSimilarityHandler() *SimilarityHandler { return &SimilarityHandler{} }

func (h *SimilarityHandler) GetByTask(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, []dto.SimilarityRecordResponse{})
}

func (h *SimilarityHandler) GetSegments(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, []dto.SegmentPairResponse{})
}

func (h *SimilarityHandler) UpdateDecision(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, dto.SuccessResponse{Message: "Decision recorded"})
}
