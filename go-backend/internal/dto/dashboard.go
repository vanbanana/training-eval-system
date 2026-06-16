package dto

// AdminDashboardResponse is the dashboard response for admin users.
type AdminDashboardResponse struct {
	TotalUsers   int64 `json:"total_users"`
	TotalTasks   int64 `json:"total_tasks"`
	TotalUploads int64 `json:"total_uploads"`
	ActiveUsers  int64 `json:"active_users"`
	TodayLogins  int64 `json:"today_logins"`
}

// TeacherDashboardResponse is the dashboard response for teacher users.
type TeacherDashboardResponse struct {
	TaskCount      int64         `json:"task_count"`
	PendingGrading int64         `json:"pending_grading"`
	StudentCount   int64         `json:"student_count"`
	RecentTasks    []TaskBrief   `json:"recent_tasks"`
	Activity7D     []DayActivity `json:"activity_7d"`
}

// TaskBrief is a brief task summary for dashboard.
type TaskBrief struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DayActivity is a single day's activity count.
type DayActivity struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// StudentDashboardResponse is the dashboard response for student users.
type StudentDashboardResponse struct {
	TaskCount      int64     `json:"task_count"`
	SubmittedCount int64     `json:"submitted_count"`
	AverageScore   *float64  `json:"average_score"`
	RecentScores   []float64 `json:"recent_scores"`
}
