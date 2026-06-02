package dto

// StudentProfileResponse is the response for GET /api/profiles/student/{userId}.
type StudentProfileResponse struct {
	ID                    int64  `json:"id"`
	StudentID             int64  `json:"student_id"`
	RadarData             any    `json:"radar_data"`
	WeaknessList          any    `json:"weakness_list"`
	Suggestions           any    `json:"suggestions"`
	ScoreTrend            any    `json:"score_trend"`
	SourceEvaluationCount int    `json:"source_evaluation_count"`
	ComputedAt            string `json:"computed_at"`
}

// SchoolProfileResponse is the response for GET /api/profiles/school.
type SchoolProfileResponse struct {
	TotalStudents        int     `json:"total_students"`
	AverageScore         float64 `json:"average_score"`
	CompletionRate       float64 `json:"completion_rate"`
	ScoreDistribution    []int   `json:"score_distribution"`
	TopDimensions        []any   `json:"top_dimensions"`
	LLMSummary           string  `json:"llm_summary,omitempty"`             // requirement 14.4
	CommonWeaknesses     []any   `json:"common_weaknesses,omitempty"`        // requirement 14.4
	RecommendTeachingFor []string `json:"recommend_teaching_for,omitempty"` // >30% students <60
}

// CourseProfileResponse is the response for GET /api/profiles/course/{courseId}.
type CourseProfileResponse struct {
	CourseID             int64   `json:"course_id"`
	CourseName           string  `json:"course_name"`
	TotalStudents        int     `json:"total_students"`
	AverageScore         float64 `json:"average_score"`
	ScoreDistribution    []int   `json:"score_distribution"`
	CompletionRate       float64 `json:"completion_rate"`
	ClassComparisons     []any   `json:"class_comparisons"`
	LLMSummary           string  `json:"llm_summary,omitempty"`             // requirement 14.4
	CommonWeaknesses     []any   `json:"common_weaknesses,omitempty"`        // requirement 14.4
	RecommendTeachingFor []string `json:"recommend_teaching_for,omitempty"` // >30% students <60
}
