package service

import (
	"context"
	"sort"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/similarity"
)

// TemplateRecommender suggests evaluation templates based on text similarity.
type TemplateRecommender struct {
	templateRepo repository.TemplateRepo
}

// NewTemplateRecommender creates a new recommender.
func NewTemplateRecommender(repo repository.TemplateRepo) *TemplateRecommender {
	return &TemplateRecommender{templateRepo: repo}
}

// RecommendParams holds input for template recommendation.
type RecommendParams struct {
	CourseID    *int64
	TeacherID   int64
	Description string
}

// MaxRecommendations is the maximum number of templates to return.
const MaxRecommendations = 5

// MaxHammingDistance is the threshold; templates with distance >= this are excluded.
const MaxHammingDistance = 20

// Recommend returns up to 5 templates most similar to the given task description.
func (tr *TemplateRecommender) Recommend(ctx context.Context, params RecommendParams) ([]model.EvalTemplate, error) {
	if params.Description == "" {
		return nil, nil
	}

	// Load templates: system + team + teacher's own
	// We load all and filter in-memory for simplicity (template count is small)
	allTemplates, err := tr.templateRepo.List(ctx, nil, params.CourseID, nil)
	if err != nil {
		return nil, err
	}

	// Filter by visibility rules
	var candidates []model.EvalTemplate
	for _, t := range allTemplates {
		switch t.Visibility {
		case "system", "team":
			candidates = append(candidates, t)
		case "private":
			if t.OwnerID != nil && *t.OwnerID == params.TeacherID {
				candidates = append(candidates, t)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Compute SimHash of the task description
	taskHash := similarity.SimHash(params.Description)

	// Score each template by hamming distance
	type scored struct {
		template model.EvalTemplate
		distance int
	}
	var results []scored

	for _, t := range candidates {
		templateHash := similarity.SimHash(t.Description)
		dist := similarity.HammingDistance(taskHash, templateHash)

		if dist < MaxHammingDistance {
			results = append(results, scored{template: t, distance: dist})
		}
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Sort by distance ascending (most similar first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].distance < results[j].distance
	})

	// Limit to MaxRecommendations
	if len(results) > MaxRecommendations {
		results = results[:MaxRecommendations]
	}

	// Extract templates
	out := make([]model.EvalTemplate, len(results))
	for i, r := range results {
		out[i] = r.template
	}

	return out, nil
}
