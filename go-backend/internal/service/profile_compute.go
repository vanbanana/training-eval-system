package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/worker"
)

// dimScoreEntry holds a single dimension score with name for radar computation.
type dimScoreEntry struct {
	name      string
	score     float64
	evalID    int64
	createdAt time.Time
	rationale string
}

// ProfileComputer handles asynchronous student profile recomputation.
type ProfileComputer struct {
	evalRepo    repository.EvaluationRepo
	profileRepo repository.ProfileRepo
	taskRepo    repository.TaskRepo
	pool        *worker.Pool
llmClient   llm.LLMClient
	}
	
	// NewProfileComputer creates a new profile computer.
	func NewProfileComputer(evalRepo repository.EvaluationRepo, profileRepo repository.ProfileRepo, taskRepo repository.TaskRepo, pool *worker.Pool) *ProfileComputer {
	return &ProfileComputer{
		evalRepo:    evalRepo,
		profileRepo: profileRepo,
		taskRepo:    taskRepo,
		pool:        pool,
	}
}

// SetLLMClient sets the LLM client for generating weakness descriptions and suggestions.
func (pc *ProfileComputer) SetLLMClient(client llm.LLMClient) {
	pc.llmClient = client
}

// TriggerRecompute submits an async profile recomputation task for a student.
func (pc *ProfileComputer) TriggerRecompute(studentID int64) {
	task := &worker.Task{
		ID: fmt.Sprintf("profile-%d", studentID),
		Fn: func(ctx context.Context) error {
			return pc.ComputeProfile(ctx, studentID)
		},
	}
	if err := pc.pool.Submit(task); err != nil {
		slog.Warn("profile_compute: submit failed", "student_id", studentID, "error", err.Error())
	}
}

// ComputeProfile performs the actual computation, including LLM-based weakness analysis.
func (pc *ProfileComputer) ComputeProfile(ctx context.Context, studentID int64) error {
	params := repository.EvalListParams{
		ListParams: repository.ListParams{Page: 1, PageSize: 1000},
		StudentID:  &studentID,
	}
	evals, _, err := pc.evalRepo.List(ctx, params)
	if err != nil {
		return fmt.Errorf("profile_compute: list evals: %w", err)
	}

	var validEvals []model.Evaluation
	for _, e := range evals {
		if e.Status == "scored" || e.Status == "confirmed" {
			validEvals = append(validEvals, e)
		}
	}

	if len(validEvals) == 0 {
		return nil
	}

	var allDimScores []dimScoreEntry
	var scoreTrend []map[string]any
	taskSummaries := make([]string, 0)

	for _, e := range validEvals {
		full, err := pc.evalRepo.GetByID(ctx, e.ID)
		if err != nil {
			continue
		}

		dims, err := pc.taskRepo.GetDimensions(ctx, e.TaskID)
		if err != nil {
			continue
		}
		dimNameMap := make(map[int64]string)
		task, _ := pc.taskRepo.GetByID(ctx, e.TaskID)
		taskName := ""
		if task != nil {
			taskName = task.Name
		}

		for _, s := range full.Scores {
			score := 0.0
			if s.TeacherScore != nil {
				score = *s.TeacherScore
			} else if s.AIScore != nil {
				score = *s.AIScore
			}
			name := dimNameMap[s.DimensionID]
			if name == "" {
				for _, d := range dims {
					if d.ID == s.DimensionID {
						name = d.Name
						break
					}
				}
			}
			if name != "" {
				allDimScores = append(allDimScores, dimScoreEntry{
					name:      name,
					score:     score,
					evalID:    e.ID,
					createdAt: e.CreatedAt,
					rationale: s.Rationale,
				})
			}
		}

		if full.TotalScore != nil {
			scoreTrend = append(scoreTrend, map[string]any{
				"date":  e.CreatedAt.Format("2006-01-02"),
				"score": *full.TotalScore,
				"task":  taskName,
			})
			taskSummaries = append(taskSummaries, fmt.Sprintf("任务「%s」: 总分=%.1f", taskName, *full.TotalScore))
		}
	}

	radarData := computeRadarFromEntries(allDimScores)
	weaknessList := ComputeWeaknessList(radarData)

	sort.Slice(scoreTrend, func(i, j int) bool {
		return scoreTrend[i]["date"].(string) < scoreTrend[j]["date"].(string)
	})

	// LLM-generated weakness descriptions and learning suggestions
	if pc.llmClient != nil && len(validEvals) >= 3 {
		suggestions := pc.generateWeaknessSuggestions(ctx, allDimScores, weaknessList, taskSummaries)
		if suggestions != nil {
			for i, w := range weaknessList {
				if s, ok := suggestions[w["name"].(string)]; ok {
					w["suggestion"] = s
					weaknessList[i] = w
				}
			}
		}
	}

	profile := &model.StudentProfile{
		StudentID:             studentID,
		RadarData:             radarData,
		WeaknessList:          weaknessList,
		ScoreTrend:            scoreTrend,
		SourceEvaluationCount: len(validEvals),
		ComputedAt:            time.Now(),
	}

	if err := pc.profileRepo.Upsert(ctx, profile); err != nil {
		return fmt.Errorf("profile_compute: upsert: %w", err)
	}

	slog.Info("profile_compute: done", "student_id", studentID, "evaluations", len(validEvals))
	return nil
}

// generateWeaknessSuggestions calls LLM to produce personalized learning suggestions.
func (pc *ProfileComputer) generateWeaknessSuggestions(ctx context.Context, scores []dimScoreEntry, weaknesses []map[string]any, taskSummaries []string) map[string]string {
	if pc.llmClient == nil || len(weaknesses) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("你是一位实训教学分析专家。以下学生的历次评价数据反映了其在各维度的表现。\n\n")

	dimScores := make(map[string][]float64)
	dimRationale := make(map[string][]string)
	for _, s := range scores {
		dimScores[s.name] = append(dimScores[s.name], s.score)
		dimRationale[s.name] = append(dimRationale[s.name], s.rationale)
	}

	sb.WriteString("## 各维度得分汇总\n")
	for name, vals := range dimScores {
		avg := average(vals)
		sb.WriteString(fmt.Sprintf("- %s: 平均分=%.1f (次数=%d)\n", name, avg, len(vals)))
		if rs, ok := dimRationale[name]; ok && len(rs) > 0 {
			sb.WriteString(fmt.Sprintf("  典型评语: %s\n", rs[len(rs)-1]))
		}
	}

	sb.WriteString("\n## 历次任务\n")
	for _, s := range taskSummaries {
		sb.WriteString(fmt.Sprintf("- %s\n", s))
	}

	if len(weaknesses) > 0 {
		sb.WriteString("\n## 已识别的薄弱维度（平均分<60）\n")
		for _, w := range weaknesses {
			sb.WriteString(fmt.Sprintf("- %s (当前掌握度: %.0f)\n", w["name"], w["score"]))
		}
	}

	sb.WriteString("\n请为每个薄弱维度生成以下内容（JSON格式）：\n")
	sb.WriteString("1. 具体的薄弱点描述（指出该维度的具体问题）\n")
	sb.WriteString("2. 个性化学习建议（不少于100字，包含可执行的学习路径或推荐资源）\n\n")
	sb.WriteString(`输出格式: {"suggestions": [{"dimension":"名称", "description":"薄弱点描述", "advice":"学习建议(≥100字)"}]}`)

	messages := []llm.ChatMessage{
		llm.NewTextMessage("system", "你是一位实训教学分析专家。请基于学生历次评价数据分析薄弱点并生成学习建议。严格以JSON格式输出。"),
		llm.NewTextMessage("user", sb.String()),
	}

	resp, err := pc.llmClient.Complete(ctx, messages, nil)
	if err != nil {
		slog.Warn("profile_compute: LLM call failed for suggestions", "error", err.Error())
		return nil
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return nil
	}

	content := resp.Choices[0].Message.Content
	var result struct {
		Suggestions []struct {
			Dimension   string `json:"dimension"`
			Description string `json:"description"`
			Advice      string `json:"advice"`
		} `json:"suggestions"`
	}

	// Parse LLM response; handle markdown-wrapped JSON
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		extracted := extractJSON(content)
		if extracted != "" {
			if err2 := json.Unmarshal([]byte(extracted), &result); err2 != nil {
				slog.Warn("profile_compute: parse LLM suggestion failed", "error", err.Error(), "extract_error", err2.Error())
				return nil
			}
		} else {
			slog.Warn("profile_compute: parse LLM suggestion failed", "error", err.Error(), "content_len", len(content))
			return nil
		}
	}

	output := make(map[string]string)
	for _, s := range result.Suggestions {
		if s.Advice != "" && len([]rune(s.Advice)) >= 50 {
			output[s.Dimension] = fmt.Sprintf("【薄弱点描述】%s\n\n【学习建议】%s", s.Description, s.Advice)
		}
	}

	return output
}

// computeRadarFromEntries computes average score per dimension name.
func computeRadarFromEntries(scores []dimScoreEntry) map[string]float64 {
	sums := make(map[string]float64)
	counts := make(map[string]int)
	for _, s := range scores {
		sums[s.name] += s.score
		counts[s.name]++
	}
	result := make(map[string]float64)
	for name, sum := range sums {
		result[name] = sum / float64(counts[name])
	}
	return result
}

// ComputeRadarData computes average score per dimension name (exported for testing).
func ComputeRadarData(scores map[string][]float64) map[string]float64 {
	result := make(map[string]float64)
	for name, vals := range scores {
		if len(vals) == 0 {
			continue
		}
		var sum float64
		for _, v := range vals {
			sum += v
		}
		result[name] = sum / float64(len(vals))
	}
	return result
}

// ComputeWeaknessList returns dimensions with average < 60.
func ComputeWeaknessList(radarData map[string]float64) []map[string]any {
	var list []map[string]any
	for name, avg := range radarData {
		if avg < 60 {
			list = append(list, map[string]any{"name": name, "score": avg})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i]["score"].(float64) < list[j]["score"].(float64)
	})
	return list
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// extractJSON extracts a JSON object from markdown-wrapped LLM response.
func extractJSON(s string) string {
	for _, pair := range []struct{ open, close string }{
		{"```json\n", "\n```"},
		{"```\n", "\n```"},
		{"```json", "```"},
		{"```", "```"},
	} {
		i := strings.Index(s, pair.open)
		if i < 0 {
			continue
		}
		j := strings.LastIndex(s, pair.close)
		if j <= i+len(pair.open) {
			continue
		}
		candidate := strings.TrimSpace(s[i+len(pair.open) : j])
		if len(candidate) > 0 && (candidate[0] == '{' || candidate[0] == '[') {
			return candidate
		}
	}
	// Fallback: find outermost {}
	if braceStart := strings.Index(s, "{"); braceStart >= 0 {
		if braceEnd := strings.LastIndex(s, "}"); braceEnd > braceStart {
			return s[braceStart : braceEnd+1]
		}
	}
	return ""
}
