package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/objectstore"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

const maxResumeSize = int64(20 * 1024 * 1024)

var resumeStatuses = map[string]struct{}{
	"draft": {}, "uploading": {}, "pending": {}, "processing": {}, "completed": {}, "failed": {},
}

var resumeSorts = map[string]string{
	"updated_at_desc": "r.updated_at DESC, r.id DESC",
	"updated_at_asc":  "r.updated_at ASC, r.id ASC",
}

func createResumeUpload(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.CreateResumeUploadRequest,
) (*types.CreateResumeUploadResponse, error) {
	title, err := validateTitle("title", req.Title)
	if err != nil {
		return nil, err
	}
	fileName := strings.TrimSpace(req.FileName)
	if req.SizeBytes <= 0 {
		return nil, requestEntityTooLarge("RESUME_FILE_TOO_LARGE", "简历文件不能为空")
	}
	if req.SizeBytes > maxResumeSize {
		return nil, requestEntityTooLarge("RESUME_FILE_TOO_LARGE", "简历文件不能超过 20 MiB")
	}
	ext, contentType, err := validateResumeDeclaration(fileName, req.ContentType)
	if err != nil {
		return nil, err
	}
	if err := objectstore.EnsureBucket(
		ctx,
		svcCtx.ObjectStore,
		svcCtx.Config.Runtime.ObjectStore.Bucket,
		svcCtx.Config.Runtime.ObjectStore.Region,
	); err != nil {
		return nil, err
	}

	resumeID := uuid.NewString()
	versionID := uuid.NewString()
	objectKey := fmt.Sprintf("users/%s/resumes/%s/%s%s", userID, resumeID, versionID, ext)
	expires := 15 * time.Minute
	putURL, err := svcCtx.UploadSigner.PresignedPutObject(
		ctx,
		svcCtx.Config.Runtime.ObjectStore.Bucket,
		objectKey,
		expires,
	)
	if err != nil {
		return nil, fmt.Errorf("sign resume upload: %w", err)
	}

	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO resumes (id, user_id, title, status)
		VALUES ($1, $2, $3, 'uploading')`,
		resumeID,
		userID,
		title,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO resume_versions (
			id, resume_id, version_no, object_key, original_filename, content_type, size_bytes
		)
		VALUES ($1, $2, 1, $3, $4, $5, $6)`,
		versionID,
		resumeID,
		objectKey,
		fileName,
		contentType,
		req.SizeBytes,
	)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE resumes SET current_version_id = $2 WHERE id = $1`, resumeID, versionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &types.CreateResumeUploadResponse{
		ResumeId:      resumeID,
		VersionId:     versionID,
		UploadUrl:     putURL.String(),
		UploadHeaders: map[string]string{"Content-Type": contentType},
		ExpiresAt:     formatTime(time.Now().UTC().Add(expires)),
	}, nil
}

func validateResumeDeclaration(fileName, contentType string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	declared := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	expected := map[string]string{
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".txt":  "text/plain",
	}
	expectedType, ok := expected[ext]
	if !ok {
		return "", "", unsupportedMedia(
			"RESUME_FILE_TYPE_UNSUPPORTED",
			"仅支持 PDF、DOCX 或 TXT 简历",
		)
	}
	if declared != expectedType && declared != "application/octet-stream" {
		return "", "", unsupportedMedia(
			"RESUME_FILE_TYPE_UNSUPPORTED",
			"文件扩展名与声明类型不一致",
		)
	}
	if declared == "application/octet-stream" {
		declared = expectedType
	}
	return ext, declared, nil
}

func validateUploadedResume(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	objectKey, fileName string,
	expectedSize int64,
) error {
	info, err := svcCtx.ObjectStore.StatObject(
		ctx,
		svcCtx.Config.Runtime.ObjectStore.Bucket,
		objectKey,
		minio.StatObjectOptions{},
	)
	if err != nil {
		return resourceNotFound("RESUME_UPLOAD_NOT_FOUND", "未找到已上传的简历文件", err)
	}
	if info.Size != expectedSize {
		return conflict(
			"RESUME_UPLOAD_SIZE_MISMATCH",
			"上传文件大小与申请凭证时不一致",
			map[string]any{"expected_size_bytes": expectedSize, "actual_size_bytes": info.Size},
		)
	}
	object, err := svcCtx.ObjectStore.GetObject(
		ctx,
		svcCtx.Config.Runtime.ObjectStore.Bucket,
		objectKey,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return resourceNotFound("RESUME_UPLOAD_NOT_FOUND", "未找到已上传的简历文件", err)
	}
	defer object.Close()
	data, err := io.ReadAll(io.LimitReader(object, maxResumeSize+1))
	if err != nil {
		return err
	}
	if int64(len(data)) != expectedSize || int64(len(data)) > maxResumeSize {
		return conflict("RESUME_UPLOAD_SIZE_MISMATCH", "上传文件大小不一致", nil)
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	valid := false
	switch ext {
	case ".pdf":
		valid = bytes.HasPrefix(bytes.TrimSpace(data), []byte("%PDF-"))
	case ".txt":
		valid = utf8.Valid(data) && !bytes.Contains(data, []byte{0})
	case ".docx":
		valid = isDOCX(data)
	}
	if !valid {
		detected := http.DetectContentType(data[:min(len(data), 512)])
		return unsupportedMedia(
			"RESUME_FILE_TYPE_UNSUPPORTED",
			fmt.Sprintf("上传内容不是有效的 %s 文件（检测类型：%s）", strings.TrimPrefix(ext, "."), detected),
		)
	}
	return nil
}

func isDOCX(data []byte) bool {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	hasContentTypes := false
	hasDocument := false
	for _, file := range reader.File {
		switch file.Name {
		case "[Content_Types].xml":
			hasContentTypes = true
		case "word/document.xml":
			hasDocument = true
		}
	}
	return hasContentTypes && hasDocument
}

type resumeParseTaskState struct {
	TaskID string
	Status string
}

func ensureResumeParseTask(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID, versionID string,
	reuseSucceeded bool,
) (resumeParseTaskState, error) {
	statuses := []string{"pending", "running"}
	if reuseSucceeded {
		statuses = append(statuses, "succeeded")
	}
	var existingID, existingStatus string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT id::text, status::text
		FROM async_tasks
		WHERE user_id = $1
		  AND ref_id = $2
		  AND task_type = 'resume.parse'
		  AND status::text = ANY($3::text[])
		ORDER BY created_at DESC, id DESC
		LIMIT 1`,
		userID,
		versionID,
		statuses,
	).Scan(&existingID, &existingStatus)
	if err == nil {
		return resumeParseTaskState{TaskID: existingID, Status: existingStatus}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return resumeParseTaskState{}, err
	}

	taskID := uuid.NewString()
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return resumeParseTaskState{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO async_tasks (id, user_id, task_type, ref_id, status, progress)
		VALUES ($1, $2, 'resume.parse', $3, 'pending', 0)`,
		taskID,
		userID,
		versionID,
	)
	if err != nil {
		var concurrentID, concurrentStatus string
		queryErr := svcCtx.Database.QueryRow(ctx, `
			SELECT id::text, status::text
			FROM async_tasks
			WHERE user_id = $1
			  AND ref_id = $2
			  AND task_type = 'resume.parse'
			  AND status IN ('pending', 'running')
			ORDER BY created_at DESC, id DESC
			LIMIT 1`,
			userID,
			versionID,
		).Scan(&concurrentID, &concurrentStatus)
		if queryErr == nil {
			return resumeParseTaskState{TaskID: concurrentID, Status: concurrentStatus}, nil
		}
		return resumeParseTaskState{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE resumes
		SET status = 'pending', updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		resumeID,
		userID,
	)
	if err != nil {
		return resumeParseTaskState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return resumeParseTaskState{}, err
	}

	task, err := sharedtasks.NewResumeParseTask(sharedtasks.ResumeParsePayload{
		TaskID:    taskID,
		ResumeID:  resumeID,
		VersionID: versionID,
		UserID:    userID,
	})
	if err == nil {
		_, err = svcCtx.TaskClient.EnqueueContext(ctx, task, asynq.Queue("heavy"))
	}
	if err != nil {
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE async_tasks
			SET status = 'failed',
			    error_code = 'TASK_ENQUEUE_FAILED',
			    error_summary = '任务暂时无法启动，请重试',
			    error_message = $2,
			    completed_at = now(),
			    updated_at = now()
			WHERE id = $1`,
			taskID,
			err.Error(),
		)
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE resumes SET status = 'failed', updated_at = now() WHERE id = $1`,
			resumeID,
		)
		return resumeParseTaskState{}, err
	}
	return resumeParseTaskState{TaskID: taskID, Status: "pending"}, nil
}

func scanResumeSummary(row pgx.Row) (types.ResumeSummaryResponse, error) {
	var item types.ResumeSummaryResponse
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.Id,
		&item.Title,
		&item.Status,
		&item.VersionId,
		&item.FileName,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return item, err
	}
	item.CreatedAt = formatTime(createdAt)
	item.UpdatedAt = formatTime(updatedAt)
	return item, nil
}

func listResumes(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	statuses []string,
	page, pageSize int,
	sortValue string,
) (*types.ResumePageResponse, error) {
	page, pageSize, offset, err := pageParams(page, pageSize)
	if err != nil {
		return nil, err
	}
	statuses, err = parseEnumFilter("status", statuses, resumeStatuses)
	if err != nil {
		return nil, err
	}
	orderBy, err := sortClause(sortValue, "updated_at_desc", resumeSorts)
	if err != nil {
		return nil, err
	}
	if statuses == nil {
		statuses = []string{}
	}
	response := &types.ResumePageResponse{
		Items:    []types.ResumeSummaryResponse{},
		Page:     page,
		PageSize: pageSize,
	}
	err = svcCtx.Database.QueryRow(ctx, `
		SELECT count(*)
		FROM resumes
		WHERE user_id = $1
		  AND (cardinality($2::text[]) = 0 OR status::text = ANY($2::text[]))`,
		userID,
		statuses,
	).Scan(&response.Total)
	if err != nil {
		return nil, err
	}
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT r.id::text,
		       r.title,
		       r.status::text,
		       COALESCE(rv.id::text, ''),
		       COALESCE(rv.original_filename, ''),
		       r.created_at,
		       r.updated_at
		FROM resumes AS r
		LEFT JOIN resume_versions AS rv ON rv.id = r.current_version_id
		WHERE r.user_id = $1
		  AND (cardinality($2::text[]) = 0 OR r.status::text = ANY($2::text[]))
		ORDER BY `+orderBy+`
		LIMIT $3 OFFSET $4`,
		userID,
		statuses,
		pageSize,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanResumeSummary(rows)
		if err != nil {
			return nil, err
		}
		response.Items = append(response.Items, item)
	}
	return response, rows.Err()
}

func loadResume(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID string,
) (*types.ResumeDetailResponse, error) {
	response := &types.ResumeDetailResponse{Facts: []types.ResumeFactResponse{}}
	var createdAt, updatedAt time.Time
	var uploadedAt *time.Time
	var processedAt *time.Time
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT r.id::text,
		       r.title,
		       r.status::text,
		       COALESCE(rv.id::text, ''),
		       COALESCE(rv.original_filename, ''),
		       r.created_at,
		       r.updated_at,
		       COALESCE(rv.version_no, 0),
		       COALESCE(rv.content_type, ''),
		       COALESCE(rv.size_bytes, 0),
		       rv.created_at,
		       rv.processed_at,
		       COALESCE(rv.parse_error, '')
		FROM resumes AS r
		LEFT JOIN resume_versions AS rv ON rv.id = r.current_version_id
		WHERE r.id = $1 AND r.user_id = $2`,
		resumeID,
		userID,
	).Scan(
		&response.Id,
		&response.Title,
		&response.Status,
		&response.VersionId,
		&response.FileName,
		&createdAt,
		&updatedAt,
		&response.VersionNo,
		&response.ContentType,
		&response.SizeBytes,
		&uploadedAt,
		&processedAt,
		&response.ParseError,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("RESUME_NOT_FOUND", "未找到该简历", err)
	}
	if err != nil {
		return nil, err
	}
	response.CreatedAt = formatTime(createdAt)
	response.UpdatedAt = formatTime(updatedAt)
	response.UploadedAt = formatOptionalTime(uploadedAt)
	response.ProcessedAt = formatOptionalTime(processedAt)
	if response.VersionId == "" {
		return response, nil
	}
	rows, err := svcCtx.Database.Query(ctx, `
		SELECT fact_type, fact_key, fact_value, source_excerpt, confidence::float8
		FROM resume_facts
		WHERE resume_version_id = $1
		ORDER BY fact_type ASC, created_at ASC, id ASC`,
		response.VersionId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var fact types.ResumeFactResponse
		var raw []byte
		if err := rows.Scan(&fact.Type, &fact.Key, &raw, &fact.Excerpt, &fact.Confidence); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &fact.Value); err != nil {
			return nil, err
		}
		response.Facts = append(response.Facts, fact)
	}
	return response, rows.Err()
}

func updateResumeTitle(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID, title string,
) (*types.ResumeSummaryResponse, error) {
	title, err := validateTitle("title", title)
	if err != nil {
		return nil, err
	}
	item, err := scanResumeSummary(svcCtx.Database.QueryRow(ctx, `
		UPDATE resumes AS r
		SET title = $3, updated_at = now()
		WHERE r.id = $1
		  AND r.user_id = $2
		RETURNING r.id::text,
		          r.title,
		          r.status::text,
		          COALESCE(r.current_version_id::text, ''),
		          COALESCE(
		              (SELECT original_filename FROM resume_versions WHERE id = r.current_version_id),
		              ''
		          ),
		          r.created_at,
		          r.updated_at`,
		resumeID,
		userID,
		title,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("RESUME_NOT_FOUND", "未找到该简历", err)
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

type resumeObject struct {
	VersionID string
	ObjectKey string
}

func deleteResume(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID string,
) error {
	tx, err := svcCtx.Database.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM resumes
		WHERE id = $1 AND user_id = $2
		FOR UPDATE`,
		resumeID,
		userID,
	).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var interviewCount int
	err = tx.QueryRow(ctx, `
		SELECT count(*) FROM interview_sessions WHERE resume_id = $1`,
		resumeID,
	).Scan(&interviewCount)
	if err != nil {
		return err
	}
	if interviewCount > 0 {
		return conflict(
			"RESUME_IN_USE",
			"这份简历已用于面试，暂时不能删除",
			map[string]any{"interview_count": interviewCount},
		)
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text, object_key
		FROM resume_versions
		WHERE resume_id = $1`,
		resumeID,
	)
	if err != nil {
		return err
	}
	objects := []resumeObject{}
	for rows.Next() {
		var object resumeObject
		if err := rows.Scan(&object.VersionID, &object.ObjectKey); err != nil {
			rows.Close()
			return err
		}
		objects = append(objects, object)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM resumes WHERE id = $1 AND user_id = $2`, resumeID, userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, object := range objects {
		err := svcCtx.ObjectStore.RemoveObject(
			ctx,
			svcCtx.Config.Runtime.ObjectStore.Bucket,
			object.ObjectKey,
			minio.RemoveObjectOptions{},
		)
		if err != nil {
			scheduleObjectCleanup(ctx, svcCtx, userID, object)
		}
	}
	return nil
}

func scheduleObjectCleanup(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	object resumeObject,
) {
	taskID := uuid.NewString()
	result, _ := json.Marshal(map[string]string{"object_key": object.ObjectKey})
	_, err := svcCtx.Database.Exec(ctx, `
		INSERT INTO async_tasks (
			id, user_id, task_type, ref_id, status, progress, result
		)
		VALUES ($1, $2, 'object.cleanup', $3, 'pending', 0, $4)`,
		taskID,
		userID,
		object.VersionID,
		result,
	)
	if err != nil {
		return
	}
	task, err := sharedtasks.NewObjectCleanupTask(sharedtasks.ObjectCleanupPayload{
		TaskID:    taskID,
		UserID:    userID,
		ObjectKey: object.ObjectKey,
	})
	if err == nil {
		_, err = svcCtx.TaskClient.EnqueueContext(ctx, task, asynq.Queue("default"))
	}
	if err != nil {
		_, _ = svcCtx.Database.Exec(ctx, `
			UPDATE async_tasks
			SET status = 'failed',
			    error_code = 'TASK_ENQUEUE_FAILED',
			    error_summary = '文件清理任务暂时无法启动',
			    error_message = $2,
			    completed_at = now(),
			    updated_at = now()
			WHERE id = $1`,
			taskID,
			err.Error(),
		)
	}
}

func completeResumeUpload(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID, versionID string,
) (*types.ResumeDetailResponse, error) {
	var objectKey, fileName string
	var expectedSize int64
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT version.object_key,
		       version.original_filename,
		       version.size_bytes
		FROM resumes AS resume
		JOIN resume_versions AS version
		  ON version.id = $2
		 AND version.resume_id = resume.id
		WHERE resume.id = $1
		  AND resume.user_id = $3`,
		resumeID,
		versionID,
		userID,
	).Scan(&objectKey, &fileName, &expectedSize)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("RESUME_VERSION_NOT_FOUND", "未找到该简历版本", err)
	}
	if err != nil {
		return nil, err
	}
	if err := validateUploadedResume(ctx, svcCtx, objectKey, fileName, expectedSize); err != nil {
		return nil, err
	}
	if _, err := ensureResumeParseTask(ctx, svcCtx, userID, resumeID, versionID, true); err != nil {
		return nil, err
	}
	return loadResume(ctx, svcCtx, userID, resumeID)
}

func reparseResume(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID, resumeID string,
) (*types.ResumeDetailResponse, error) {
	var versionID, objectKey string
	err := svcCtx.Database.QueryRow(ctx, `
		SELECT version.id::text, version.object_key
		FROM resumes AS resume
		JOIN resume_versions AS version ON version.id = resume.current_version_id
		WHERE resume.id = $1 AND resume.user_id = $2`,
		resumeID,
		userID,
	).Scan(&versionID, &objectKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, resourceNotFound("RESUME_NOT_FOUND", "未找到可重新解析的简历", err)
	}
	if err != nil {
		return nil, err
	}
	if _, err := svcCtx.ObjectStore.StatObject(
		ctx,
		svcCtx.Config.Runtime.ObjectStore.Bucket,
		objectKey,
		minio.StatObjectOptions{},
	); err != nil {
		return nil, resourceNotFound("RESUME_UPLOAD_NOT_FOUND", "简历原始文件不存在", err)
	}
	if _, err := ensureResumeParseTask(ctx, svcCtx, userID, resumeID, versionID, false); err != nil {
		return nil, err
	}
	return loadResume(ctx, svcCtx, userID, resumeID)
}
