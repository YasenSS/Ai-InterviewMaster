package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

func ASRHandler(db *pgxpool.Pool, store *minio.Client, bucket, endpoint string) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload sharedtasks.ASRPayload;if err:=json.Unmarshal(task.Payload(),&payload);err!=nil{return err};_,_=db.Exec(ctx,`UPDATE async_tasks SET status='running',progress=10,updated_at=now() WHERE id=$1`,payload.TaskID)
		object,err:=store.GetObject(ctx,bucket,payload.ObjectKey,minio.GetObjectOptions{});if err!=nil{return err};defer object.Close();audio,err:=io.ReadAll(io.LimitReader(object,50*1024*1024));if err!=nil{return err}
		var body bytes.Buffer;writer:=multipart.NewWriter(&body);part,err:=writer.CreateFormFile("audio_file","interview-audio");if err!=nil{return err};if _,err=part.Write(audio);err!=nil{return err};_ = writer.Close()
		url:=strings.TrimRight(endpoint,"/")+"/asr?output=json";if payload.Language!=""{url+="&language="+payload.Language};req,err:=http.NewRequestWithContext(ctx,http.MethodPost,url,&body);if err!=nil{return err};req.Header.Set("Content-Type",writer.FormDataContentType());resp,err:=(&http.Client{}).Do(req);if err!=nil{return fmt.Errorf("call ASR: %w",err)};defer resp.Body.Close();if resp.StatusCode/100!=2{return fmt.Errorf("ASR returned %s",resp.Status)}
		var result map[string]any;if err=json.NewDecoder(resp.Body).Decode(&result);err!=nil{return err};encoded,_:=json.Marshal(result);_,err=db.Exec(ctx,`UPDATE async_tasks SET status='succeeded',progress=100,result=$2,completed_at=now(),updated_at=now() WHERE id=$1`,payload.TaskID,encoded);return err
	}
}
