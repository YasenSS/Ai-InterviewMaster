package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

func loadInterview(ctx context.Context, db *pgxpool.Pool, userID, sessionID string) (*types.InterviewSessionResponse, error) {
	resp := &types.InterviewSessionResponse{}
	err := db.QueryRow(ctx, `SELECT id::text,title,status::text,current_ordinal,created_at::text FROM interview_sessions WHERE id=$1 AND user_id=$2`, sessionID, userID).Scan(&resp.Id,&resp.Title,&resp.Status,&resp.CurrentOrdinal,&resp.CreatedAt)
	if err != nil { return nil, fmt.Errorf("interview not found: %w", err) }
	rows,err:=db.Query(ctx,`SELECT ordinal,question,COALESCE(answer,''),COALESCE(answered_at::text,'') FROM interview_turns WHERE session_id=$1 ORDER BY ordinal`,sessionID);if err!=nil{return nil,err};defer rows.Close()
	for rows.Next(){var turn types.InterviewTurnResponse;if err:=rows.Scan(&turn.Ordinal,&turn.Question,&turn.Answer,&turn.AnsweredAt);err!=nil{return nil,err};resp.Turns=append(resp.Turns,turn)}
	return resp,rows.Err()
}

func encodeStrings(values []string) []byte { data,_:=json.Marshal(values);return data }
