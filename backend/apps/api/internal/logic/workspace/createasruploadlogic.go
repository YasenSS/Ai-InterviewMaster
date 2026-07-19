// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/types"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/objectstore"
	"github.com/google/uuid"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateASRUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateASRUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateASRUploadLogic {
	return &CreateASRUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateASRUploadLogic) CreateASRUpload(req *types.ASRUploadRequest) (resp *types.ASRUploadResponse, err error) {
	userID,err:=currentUserID(l.ctx);if err!=nil{return nil,err};if req.SizeBytes<=0||req.SizeBytes>50*1024*1024{return nil,fmt.Errorf("audio file must be between 1 byte and 50MB")}
	ext:=strings.ToLower(filepath.Ext(req.FileName));allowed:=map[string]bool{".wav":true,".mp3":true,".m4a":true,".webm":true,".ogg":true};if !allowed[ext]{return nil,fmt.Errorf("unsupported audio format")}
	if err:=objectstore.EnsureBucket(l.ctx,l.svcCtx.ObjectStore,l.svcCtx.Config.Runtime.ObjectStore.Bucket,l.svcCtx.Config.Runtime.ObjectStore.Region);err!=nil{return nil,err};key:=fmt.Sprintf("users/%s/audio/%s%s",userID,uuid.NewString(),ext);expires:=15*time.Minute;url,err:=l.svcCtx.ObjectStore.PresignedPutObject(l.ctx,l.svcCtx.Config.Runtime.ObjectStore.Bucket,key,expires);if err!=nil{return nil,err};return &types.ASRUploadResponse{ObjectKey:key,UploadUrl:url.String(),ExpiresAt:time.Now().Add(expires).UTC().Format(time.RFC3339)},nil
}
