package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

func ResumeParseHandler(db *pgxpool.Pool, store *minio.Client, bucket, tikaURL string) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.ResumeParsePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil { return fmt.Errorf("decode task: %w", err) }
		_, _ = db.Exec(ctx, `UPDATE async_tasks SET status='running',progress=10,updated_at=now() WHERE id=$1`, payload.TaskID)
		_, _ = db.Exec(ctx, `UPDATE resumes SET status='processing',updated_at=now() WHERE id=$1`, payload.ResumeID)
		var key, contentType string
		if err := db.QueryRow(ctx, `SELECT object_key, content_type FROM resume_versions WHERE id=$1`, payload.VersionID).Scan(&key, &contentType); err != nil { return fail(ctx,db,payload,err) }
		object, err := store.GetObject(ctx,bucket,key,minio.GetObjectOptions{}); if err != nil { return fail(ctx,db,payload,err) }; defer object.Close()
		data, err := io.ReadAll(io.LimitReader(object,20*1024*1024)); if err != nil { return fail(ctx,db,payload,err) }
		text, err := extractText(ctx, data, contentType, tikaURL)
		if err != nil { return fail(ctx,db,payload,err) }
		if text=="" { return fail(ctx,db,payload,fmt.Errorf("Tika did not extract readable resume text")) }
		tx,err:=db.Begin(ctx);if err!=nil{return err};defer tx.Rollback(ctx)
		_,err=tx.Exec(ctx,`UPDATE resume_versions SET extracted_text=$2,processed_at=now() WHERE id=$1`,payload.VersionID,text);if err!=nil{return fail(ctx,db,payload,err)}
		_,err=tx.Exec(ctx,`DELETE FROM resume_facts WHERE resume_version_id=$1`,payload.VersionID);if err!=nil{return fail(ctx,db,payload,err)}
		_,err=tx.Exec(ctx,`DELETE FROM resume_chunks WHERE resume_version_id=$1`,payload.VersionID);if err!=nil{return fail(ctx,db,payload,err)}
		facts,_:=json.Marshal(map[string]string{"summary":firstLine(text)})
		_,err=tx.Exec(ctx,`INSERT INTO resume_facts(resume_version_id,fact_type,fact_key,fact_value,source_excerpt) VALUES($1,'summary','profile',$2,$3)`,payload.VersionID,facts,firstLine(text));if err!=nil{return fail(ctx,db,payload,err)}
		for i,chunk:=range chunks(text,1000){_,err=tx.Exec(ctx,`INSERT INTO resume_chunks(resume_version_id,chunk_no,content,token_count,embedding,embedding_model) VALUES($1,$2,$3,$4,$5::vector,$6)`,payload.VersionID,i+1,chunk,len(strings.Fields(chunk)),baselineEmbedding(chunk),"baseline-hash-1536");if err!=nil{return fail(ctx,db,payload,err)}}
		_,err=tx.Exec(ctx,`UPDATE resumes SET status='completed',updated_at=now() WHERE id=$1`,payload.ResumeID);if err!=nil{return fail(ctx,db,payload,err)}
		_,err=tx.Exec(ctx,`UPDATE async_tasks SET status='succeeded',progress=100,completed_at=now(),updated_at=now() WHERE id=$1`,payload.TaskID);if err!=nil{return fail(ctx,db,payload,err)}
		return tx.Commit(ctx)
	}
}
func fail(ctx context.Context,db *pgxpool.Pool,p sharedtasks.ResumeParsePayload,err error) error { _,_=db.Exec(ctx,`UPDATE async_tasks SET status='failed',error_message=$2,updated_at=now() WHERE id=$1`,p.TaskID,err.Error());_,_=db.Exec(ctx,`UPDATE resumes SET status='failed',updated_at=now() WHERE id=$1`,p.ResumeID);return err }
func firstLine(s string) string { if i:=strings.IndexByte(s,'\n');i>=0{return strings.TrimSpace(s[:i])};return s }
func chunks(s string,n int)[]string{r:=[]rune(s);var out[]string;for len(r)>0{end:=n;if len(r)<end{end=len(r)};out=append(out,string(r[:end]));r=r[end:]};return out}

func extractText(ctx context.Context, data []byte, contentType, tikaURL string) (string, error) {
	if strings.HasPrefix(contentType, "text/") { return strings.TrimSpace(string(data)), nil }
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(tikaURL, "/")+"/tika", strings.NewReader(string(data)))
	if err != nil { return "", err }; req.Header.Set("Content-Type", contentType)
	resp, err := (&http.Client{}).Do(req); if err != nil { return "", fmt.Errorf("call Tika: %w", err) }; defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return "", fmt.Errorf("Tika returned %s", resp.Status) }
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)); if err != nil { return "", err }; return strings.TrimSpace(string(body)),nil
}

func baselineEmbedding(text string) string {
	values := make([]string, 1536)
	seed := sha256.Sum256([]byte(text))
	for i := range values { values[i] = fmt.Sprintf("%.6f", (float64(seed[i%len(seed)])/127.5)-1) }
	return "[" + strings.Join(values, ",") + "]"
}
