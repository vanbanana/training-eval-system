package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/store"
)

type SQLiteProfileRepo struct{ db *store.DB }

func NewProfileRepo(db *store.DB) ProfileRepo { return &SQLiteProfileRepo{db: db} }

func (r *SQLiteProfileRepo) GetByStudentID(ctx context.Context, studentID int64) (*model.StudentProfile, error) {
	var p model.StudentProfile
	var radar, weakness, suggestions, trend sql.NullString
	var computedAt sql.NullString
	err := r.db.Reader.QueryRowContext(ctx,
		`SELECT id,student_id,radar_data,weakness_list,suggestions,score_trend,source_evaluation_count,computed_at
		 FROM student_profiles WHERE student_id=?`, studentID).Scan(
		&p.ID, &p.StudentID, &radar, &weakness, &suggestions, &trend, &p.SourceEvaluationCount, &computedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("profile_repo: get: %w", err)
	}
	if radar.Valid {
		_ = json.Unmarshal([]byte(radar.String), &p.RadarData)
	}
	if weakness.Valid {
		_ = json.Unmarshal([]byte(weakness.String), &p.WeaknessList)
	}
	if suggestions.Valid {
		_ = json.Unmarshal([]byte(suggestions.String), &p.Suggestions)
	}
	if trend.Valid {
		_ = json.Unmarshal([]byte(trend.String), &p.ScoreTrend)
	}
	p.ComputedAt = parseTime(computedAt.String)
	return &p, nil
}

func (r *SQLiteProfileRepo) Upsert(ctx context.Context, p *model.StudentProfile) error {
	radarJSON, _ := json.Marshal(p.RadarData)
	weaknessJSON, _ := json.Marshal(p.WeaknessList)
	suggestionsJSON, _ := json.Marshal(p.Suggestions)
	trendJSON, _ := json.Marshal(p.ScoreTrend)

	_, err := r.db.Writer.ExecContext(ctx,
		`INSERT INTO student_profiles (student_id,radar_data,weakness_list,suggestions,score_trend,source_evaluation_count,computed_at)
		 VALUES (?,?,?,?,?,?,datetime('now'))
		 ON CONFLICT(student_id) DO UPDATE SET
		   radar_data=excluded.radar_data, weakness_list=excluded.weakness_list,
		   suggestions=excluded.suggestions, score_trend=excluded.score_trend,
		   source_evaluation_count=excluded.source_evaluation_count, computed_at=excluded.computed_at`,
		p.StudentID, string(radarJSON), string(weaknessJSON), string(suggestionsJSON), string(trendJSON), p.SourceEvaluationCount)
	return err
}
