package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxHits = 5

type factHit struct {
	FactID        string `json:"fact_id"`
	FactType      string `json:"fact_type"`
	FactKey       string `json:"fact_key"`
	SourceExcerpt string `json:"source_excerpt"`
}

// ResumeFactsTool looks up facts for a constructor-bound user. Model args
// cannot change the tenant scope.
type ResumeFactsTool struct {
	DB     *pgxpool.Pool
	UserID string
}

func NewResumeFactsTool(db *pgxpool.Pool, userID string) *ResumeFactsTool {
	return &ResumeFactsTool{DB: db, UserID: strings.TrimSpace(userID)}
}

func (t *ResumeFactsTool) Name() string { return "lookup_resume_facts" }

func (t *ResumeFactsTool) Description() string {
	return "Search the current candidate's resume facts by keyword. Read-only. Do not pass user_id."
}

func (t *ResumeFactsTool) Schema() []byte {
	return []byte(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string","minLength":1,"maxLength":200}},"required":["query"]}`)
}

func (t *ResumeFactsTool) Call(ctx context.Context, args string) (string, error) {
	if t.DB == nil || t.UserID == "" {
		return `{"hits":[],"error":"not_configured"}`, nil
	}
	var in struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(platformai.StripTenantArgs(args)), &in); err != nil {
		return `{"hits":[],"error":"invalid_args"}`, nil
	}
	query := strings.TrimSpace(in.Query)
	if query == "" || utf8.RuneCountInString(query) > 200 {
		return `{"hits":[]}`, nil
	}
	pattern := "%" + escapeLike(query) + "%"
	rows, err := t.DB.Query(ctx, `
		SELECT fact.id::text, fact.fact_type, fact.fact_key, fact.source_excerpt
		FROM resume_facts AS fact
		JOIN resume_versions AS version ON version.id = fact.resume_version_id
		JOIN resumes AS resume ON resume.id = version.resume_id
		WHERE resume.user_id = $1
		  AND (
		        fact.fact_key ILIKE $2 ESCAPE '\'
		     OR fact.source_excerpt ILIKE $2 ESCAPE '\'
		     OR fact.fact_value::text ILIKE $2 ESCAPE '\'
		  )
		ORDER BY fact.created_at DESC
		LIMIT $3`,
		t.UserID,
		pattern,
		maxHits,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	hits := make([]factHit, 0, maxHits)
	for rows.Next() {
		var item factHit
		if err := rows.Scan(&item.FactID, &item.FactType, &item.FactKey, &item.SourceExcerpt); err != nil {
			return "", err
		}
		item.SourceExcerpt = platformai.ClipRunes(item.SourceExcerpt, 240)
		hits = append(hits, item)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{"hits": hits})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type intelHit struct {
	Company string `json:"company"`
	Role    string `json:"role"`
	Topic   string `json:"topic"`
	Summary string `json:"summary"`
}

// CompanyIntelTool searches the local curated corpus. It never scrapes the web.
type CompanyIntelTool struct {
	DB *pgxpool.Pool
}

func NewCompanyIntelTool(db *pgxpool.Pool) *CompanyIntelTool {
	return &CompanyIntelTool{DB: db}
}

func (t *CompanyIntelTool) Name() string { return "lookup_company_intel" }

func (t *CompanyIntelTool) Description() string {
	return "Search local curated company interview notes by company, role, or topic. Read-only."
}

func (t *CompanyIntelTool) Schema() []byte {
	return []byte(`{"type":"object","additionalProperties":false,"properties":{"company":{"type":"string"},"role":{"type":"string"},"topic":{"type":"string"}}}`)
}

func (t *CompanyIntelTool) Call(ctx context.Context, args string) (string, error) {
	if t.DB == nil {
		return `{"hits":[],"error":"not_configured"}`, nil
	}
	var in struct {
		Company string `json:"company"`
		Role    string `json:"role"`
		Topic   string `json:"topic"`
	}
	_ = json.Unmarshal([]byte(platformai.StripTenantArgs(args)), &in)
	hits, err := SearchIntel(ctx, t.DB, in.Company, in.Role, in.Topic, maxHits)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{"hits": hits})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func SearchIntel(ctx context.Context, db *pgxpool.Pool, company, role, topic string, limit int) ([]intelHit, error) {
	if db == nil {
		return nil, fmt.Errorf("intel store is nil")
	}
	if limit < 1 || limit > 20 {
		limit = maxHits
	}
	company = strings.TrimSpace(company)
	role = strings.TrimSpace(role)
	topic = strings.TrimSpace(topic)
	rows, err := db.Query(ctx, `
		SELECT company, role, topic, summary
		FROM public_intel_items
		WHERE ($1 = '' OR company ILIKE '%' || $1 || '%' OR topic ILIKE '%' || $1 || '%' OR summary ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR role ILIKE '%' || $2 || '%' OR topic ILIKE '%' || $2 || '%' OR summary ILIKE '%' || $2 || '%')
		  AND ($3 = '' OR topic ILIKE '%' || $3 || '%' OR summary ILIKE '%' || $3 || '%')
		ORDER BY created_at DESC
		LIMIT $4`,
		company,
		role,
		topic,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]intelHit, 0, limit)
	for rows.Next() {
		var item intelHit
		if err := rows.Scan(&item.Company, &item.Role, &item.Topic, &item.Summary); err != nil {
			return nil, err
		}
		item.Summary = platformai.ClipRunes(item.Summary, 400)
		hits = append(hits, item)
	}
	return hits, rows.Err()
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
