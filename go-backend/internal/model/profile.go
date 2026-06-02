package model

import "time"

// StudentProfile holds computed learning analytics for a student.
type StudentProfile struct {
	ID                    int64     `json:"id"`
	StudentID             int64     `json:"student_id"`
	RadarData             any       `json:"radar_data"`     // JSON
	WeaknessList          any       `json:"weakness_list"`  // JSON array
	Suggestions           any       `json:"suggestions"`    // JSON array
	ScoreTrend            any       `json:"score_trend"`    // JSON array
	SourceEvaluationCount int       `json:"source_evaluation_count"`
	ComputedAt            time.Time `json:"computed_at"`
}
