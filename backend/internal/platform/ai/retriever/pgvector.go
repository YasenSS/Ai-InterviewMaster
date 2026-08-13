package retriever

import (
	"context"
	"errors"
	"fmt"
	"strings"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

const (
	CorpusPrivate = "private_resume"
	CorpusPublic  = "public_intel"
)

type Query struct {
	UserID string
	Corpus string
	Text   string
	Limit  int
}

type Hit struct {
	ID      string  `json:"id"`
	Corpus  string  `json:"corpus"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// PGVector searches pgvector collections. Private queries always filter by
// constructor/query user_id; the model cannot widen the scope.
type PGVector struct {
	DB       *pgxpool.Pool
	Embed    platformai.EmbeddingModel
	BindUser string
}

func (r PGVector) Search(ctx context.Context, query Query) ([]Hit, error) {
	if r.DB == nil {
		return nil, errors.New("retriever database is nil")
	}
	limit := query.Limit
	if limit < 1 || limit > 8 {
		limit = 5
	}
	corpus := strings.TrimSpace(query.Corpus)
	if corpus == "" {
		corpus = CorpusPrivate
	}
	userID := strings.TrimSpace(r.BindUser)
	if userID == "" {
		userID = strings.TrimSpace(query.UserID)
	}
	text := strings.TrimSpace(query.Text)
	if text == "" {
		return []Hit{}, nil
	}

	switch corpus {
	case CorpusPrivate:
		if userID == "" {
			return nil, errors.New("private retriever queries require user_id")
		}
		return r.searchPrivate(ctx, userID, text, limit)
	case CorpusPublic:
		return r.searchPublic(ctx, text, limit)
	default:
		return nil, fmt.Errorf("unknown corpus %q", corpus)
	}
}

func (r PGVector) searchPrivate(ctx context.Context, userID, text string, limit int) ([]Hit, error) {
	if r.Embed == nil {
		return []Hit{}, nil
	}
	vectors, err := r.Embed.Embed(ctx, []string{text})
	if err != nil || len(vectors) == 0 || len(vectors[0]) == 0 {
		return []Hit{}, err
	}
	rows, err := r.DB.Query(ctx, `
		SELECT chunk.id::text, chunk.content, (chunk.embedding <=> $2) AS distance
		FROM resume_chunks AS chunk
		JOIN resume_versions AS version ON version.id = chunk.resume_version_id
		JOIN resumes AS resume ON resume.id = version.resume_id
		WHERE resume.user_id = $1
		  AND chunk.embedding IS NOT NULL
		ORDER BY distance ASC
		LIMIT $3`,
		userID,
		pgvector.NewVector(vectors[0]),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]Hit, 0, limit)
	for rows.Next() {
		var item Hit
		var distance float64
		if err := rows.Scan(&item.ID, &item.Content, &distance); err != nil {
			return nil, err
		}
		item.Corpus = CorpusPrivate
		item.Content = platformai.ClipRunes(item.Content, 400)
		item.Score = 1 - distance
		hits = append(hits, item)
	}
	return hits, rows.Err()
}

func (r PGVector) searchPublic(ctx context.Context, text string, limit int) ([]Hit, error) {
	pattern := "%" + text + "%"
	rows, err := r.DB.Query(ctx, `
		SELECT id::text, summary
		FROM public_intel_items
		WHERE company ILIKE $1 OR role ILIKE $1 OR topic ILIKE $1 OR summary ILIKE $1
		ORDER BY created_at DESC
		LIMIT $2`,
		pattern,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]Hit, 0, limit)
	for rows.Next() {
		var item Hit
		if err := rows.Scan(&item.ID, &item.Content); err != nil {
			return nil, err
		}
		item.Corpus = CorpusPublic
		item.Content = platformai.ClipRunes(item.Content, 400)
		hits = append(hits, item)
	}
	return hits, rows.Err()
}
