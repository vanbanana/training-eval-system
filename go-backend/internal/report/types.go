// Package report generates Excel and PDF evaluation reports.
package report

// TaskReportData holds all data needed to generate a task evaluation report.
type TaskReportData struct {
	TaskName   string
	CourseName string
	Dimensions []DimensionInfo
	Students   []StudentReportRow
}

// DimensionInfo describes a dimension for the report header.
type DimensionInfo struct {
	ID   int64
	Name string
}

// StudentReportRow represents one student's evaluation in the report.
type StudentReportRow struct {
	StudentName     string
	TotalScore      float64
	DimensionScores map[int64]float64 // dimensionID -> score
	TeacherComment  string
}

// ScoreDistribution holds the count of students in each score range.
type ScoreDistribution struct {
	Range0to59  int
	Range60to69 int
	Range70to79 int
	Range80to89 int
	Range90to100 int
}

// ComputeDistribution computes score distribution from student rows.
func ComputeDistribution(students []StudentReportRow) ScoreDistribution {
	var dist ScoreDistribution
	for _, s := range students {
		switch {
		case s.TotalScore < 60:
			dist.Range0to59++
		case s.TotalScore < 70:
			dist.Range60to69++
		case s.TotalScore < 80:
			dist.Range70to79++
		case s.TotalScore < 90:
			dist.Range80to89++
		default:
			dist.Range90to100++
		}
	}
	return dist
}
