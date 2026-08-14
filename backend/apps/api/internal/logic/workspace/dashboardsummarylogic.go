// Dashboard aggregation is intentionally isolated from resource mutation logic.
package workspace

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type DashboardSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDashboardSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DashboardSummaryLogic {
	return &DashboardSummaryLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DashboardSummaryLogic) DashboardSummary() (*types.DashboardSummaryResponse, error) {
	userID, err := currentUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	response := &types.DashboardSummaryResponse{
		ScoreTrend:        []types.ScoreTrendResponse{},
		ImprovementTopics: []types.ImprovementTopicResponse{},
		RecentResumes:     []types.ResumeSummaryResponse{},
		RecentInterviews:  []types.InterviewSummaryResponse{},
	}
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		SELECT
			(SELECT count(*) FROM resumes WHERE user_id = $1),
			(SELECT count(*) FROM interview_sessions WHERE user_id = $1),
			(
				SELECT count(*)
				FROM interview_sessions
				WHERE user_id = $1 AND status = 'completed'
			)`,
		userID,
	).Scan(
		&response.Counts.Resumes,
		&response.Counts.Interviews,
		&response.Counts.CompletedInterviews,
	)
	if err != nil {
		return nil, err
	}
	var averageScore *float64
	err = l.svcCtx.Database.QueryRow(l.ctx, `
		SELECT avg(report.overall_score)::float8
		FROM interview_reports AS report
		JOIN interview_sessions AS session ON session.id = report.session_id
		WHERE session.user_id = $1
		  AND session.status = 'completed'
		  AND report.status IN ('completed', 'degraded')`,
		userID,
	).Scan(&averageScore)
	if err != nil {
		return nil, err
	}
	if averageScore != nil {
		response.AverageScore = averageScore
	}
	if err := l.loadTrend(userID, response); err != nil {
		return nil, err
	}
	if err := l.loadImprovementTopics(userID, response); err != nil {
		return nil, err
	}
	if err := l.loadRecentResumes(userID, response); err != nil {
		return nil, err
	}
	if err := l.loadRecentInterviews(userID, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (l *DashboardSummaryLogic) loadTrend(userID string, response *types.DashboardSummaryResponse) error {
	rows, err := l.svcCtx.Database.Query(l.ctx, `
		SELECT to_char(
		           (session.completed_at AT TIME ZONE 'UTC')::date,
		           'YYYY-MM-DD'
		       ),
		       avg(report.overall_score)::float8,
		       count(*)::bigint
		FROM interview_reports AS report
		JOIN interview_sessions AS session ON session.id = report.session_id
		WHERE session.user_id = $1
		  AND session.status = 'completed'
		  AND report.status IN ('completed', 'degraded')
		  AND session.completed_at >= now() - interval '30 days'
		GROUP BY (session.completed_at AT TIME ZONE 'UTC')::date
		ORDER BY (session.completed_at AT TIME ZONE 'UTC')::date ASC`,
		userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item types.ScoreTrendResponse
		if err := rows.Scan(&item.Date, &item.AverageScore, &item.InterviewCount); err != nil {
			return err
		}
		response.ScoreTrend = append(response.ScoreTrend, item)
	}
	return rows.Err()
}

func (l *DashboardSummaryLogic) loadImprovementTopics(
	userID string,
	response *types.DashboardSummaryResponse,
) error {
	rows, err := l.svcCtx.Database.Query(l.ctx, `
		SELECT lower(regexp_replace(trim(topic.value), '\s+', ' ', 'g')) AS label,
		       count(*)::bigint
		FROM interview_reports AS report
		JOIN interview_sessions AS session ON session.id = report.session_id
		CROSS JOIN LATERAL jsonb_array_elements_text(report.improvements) AS topic(value)
		WHERE session.user_id = $1
		  AND report.status IN ('completed', 'degraded')
		  AND trim(topic.value) <> ''
		GROUP BY lower(regexp_replace(trim(topic.value), '\s+', ' ', 'g'))
		ORDER BY count(*) DESC, label ASC
		LIMIT 5`,
		userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item types.ImprovementTopicResponse
		if err := rows.Scan(&item.Label, &item.Count); err != nil {
			return err
		}
		response.ImprovementTopics = append(response.ImprovementTopics, item)
	}
	return rows.Err()
}

func (l *DashboardSummaryLogic) loadRecentResumes(
	userID string,
	response *types.DashboardSummaryResponse,
) error {
	rows, err := l.svcCtx.Database.Query(l.ctx, `
		SELECT resume.id::text,
		       resume.title,
		       resume.status::text,
		       COALESCE(version.id::text, ''),
		       COALESCE(version.original_filename, ''),
		       resume.created_at,
		       resume.updated_at
		FROM resumes AS resume
		LEFT JOIN resume_versions AS version ON version.id = resume.current_version_id
		WHERE resume.user_id = $1
		ORDER BY resume.updated_at DESC, resume.id DESC
		LIMIT 5`,
		userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanResumeSummary(rows)
		if err != nil {
			return err
		}
		response.RecentResumes = append(response.RecentResumes, item)
	}
	return rows.Err()
}

func (l *DashboardSummaryLogic) loadRecentInterviews(
	userID string,
	response *types.DashboardSummaryResponse,
) error {
	rows, err := l.svcCtx.Database.Query(l.ctx,
		interviewSummarySelect+`
		WHERE session.user_id = $1
		GROUP BY session.id, resume.id, qset.id, report.id
		ORDER BY session.updated_at DESC, session.id DESC
		LIMIT 5`,
		userID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanInterviewSummary(rows)
		if err != nil {
			return err
		}
		response.RecentInterviews = append(response.RecentInterviews, item)
	}
	return rows.Err()
}
