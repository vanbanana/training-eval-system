package service

import (
	"testing"

	"pgregory.net/rapid"
)

// Property 14: radar_data averaging
func TestProperty_RadarDataAveraging(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numDims := rapid.IntRange(1, 5).Draw(t, "numDims")
		numEvals := rapid.IntRange(1, 10).Draw(t, "numEvals")

		input := make(map[string][]float64)
		for i := 0; i < numDims; i++ {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dim_name")
			var scores []float64
			for j := 0; j < numEvals; j++ {
				scores = append(scores, rapid.Float64Range(0, 100).Draw(t, "score"))
			}
			input[name] = scores
		}

		result := ComputeRadarData(input)

		for name, scores := range input {
			var sum float64
			for _, s := range scores {
				sum += s
			}
			expected := sum / float64(len(scores))
			got := result[name]
			diff := expected - got
			if diff < -0.001 || diff > 0.001 {
				t.Fatalf("dimension %q: expected avg %.4f, got %.4f", name, expected, got)
			}
		}
	})
}

// Property 15: weakness identification
func TestProperty_WeaknessIdentification(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numDims := rapid.IntRange(1, 6).Draw(t, "numDims")
		radarData := make(map[string]float64)
		for i := 0; i < numDims; i++ {
			name := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "dim")
			score := rapid.Float64Range(0, 100).Draw(t, "score")
			radarData[name] = score
		}

		weaknesses := ComputeWeaknessList(radarData)

		// Verify: weakness_list contains exactly dims with avg < 60
		weakSet := make(map[string]bool)
		for _, w := range weaknesses {
			weakSet[w["name"].(string)] = true
		}

		for name, avg := range radarData {
			inList := weakSet[name]
			if avg < 60 && !inList {
				t.Fatalf("dimension %q (avg=%.2f < 60) should be in weakness_list", name, avg)
			}
			if avg >= 60 && inList {
				t.Fatalf("dimension %q (avg=%.2f >= 60) should NOT be in weakness_list", name, avg)
			}
		}
	})
}

// Unit test: radar data averaging with known values
func TestComputeRadarData_Basic(t *testing.T) {
	input := map[string][]float64{
		"代码规范": {80, 90, 70},
		"文档质量": {60, 50, 70},
	}
	result := ComputeRadarData(input)

	if result["代码规范"] != 80 {
		t.Errorf("expected 80, got %f", result["代码规范"])
	}
	if result["文档质量"] != 60 {
		t.Errorf("expected 60, got %f", result["文档质量"])
	}
}

// Unit test: weakness list
func TestComputeWeaknessList_Basic(t *testing.T) {
	radarData := map[string]float64{
		"代码规范": 85,
		"文档质量": 55,
		"并发正确性": 42,
	}
	weaknesses := ComputeWeaknessList(radarData)

	if len(weaknesses) != 2 {
		t.Fatalf("expected 2 weaknesses, got %d", len(weaknesses))
	}
	// Should be sorted by score ascending
	if weaknesses[0]["name"] != "并发正确性" {
		t.Errorf("expected '并发正确性' first (weakest), got %v", weaknesses[0]["name"])
	}
}

func TestComputeWeaknessList_NoneBelow60(t *testing.T) {
	radarData := map[string]float64{
		"A": 60, "B": 85, "C": 100,
	}
	weaknesses := ComputeWeaknessList(radarData)
	if len(weaknesses) != 0 {
		t.Errorf("expected 0 weaknesses, got %d", len(weaknesses))
	}
}
