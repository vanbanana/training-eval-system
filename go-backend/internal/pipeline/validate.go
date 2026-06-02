package pipeline

import "fmt"

// ValidateTeacherScore validates that a teacher score is in [0, 100].
func ValidateTeacherScore(score float64) error {
	if score < 0 || score > 100 {
		return fmt.Errorf("teacher score %.2f out of range [0, 100]", score)
	}
	return nil
}
