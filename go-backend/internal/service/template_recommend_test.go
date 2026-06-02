package service

import (
	"testing"

	"github.com/smartedu/training-eval-system/internal/similarity"
)

func TestTemplateRecommend_MaxResults(t *testing.T) {
	// MaxRecommendations should be 5
	if MaxRecommendations != 5 {
		t.Errorf("MaxRecommendations should be 5, got %d", MaxRecommendations)
	}
}

func TestTemplateRecommend_MaxHammingThreshold(t *testing.T) {
	// MaxHammingDistance should be 20
	if MaxHammingDistance != 20 {
		t.Errorf("MaxHammingDistance should be 20, got %d", MaxHammingDistance)
	}
}

func TestSimHash_SimilarTexts(t *testing.T) {
	// Two similar descriptions should have low hamming distance
	a := similarity.SimHash("实现生产者消费者模型，理解Java线程同步机制")
	b := similarity.SimHash("实现生产者消费者模式，学习Java多线程同步")

	dist := similarity.HammingDistance(a, b)
	if dist >= MaxHammingDistance {
		t.Errorf("similar texts should have distance < %d, got %d", MaxHammingDistance, dist)
	}
}

func TestSimHash_DifferentTexts(t *testing.T) {
	// Very different descriptions should have high hamming distance
	a := similarity.SimHash("实现生产者消费者模型，理解Java线程同步机制")
	b := similarity.SimHash("Configure Kubernetes deployment with Helm charts and Istio service mesh")

	dist := similarity.HammingDistance(a, b)
	if dist < 10 {
		t.Errorf("very different texts should have distance >= 10, got %d", dist)
	}
}
