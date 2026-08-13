// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/config"
	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/airuntime"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/cache"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/database"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/objectstore"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type ServiceContext struct {
	Config       config.Config
	Database     *pgxpool.Pool
	Redis        *redis.Client
	ObjectStore  *minio.Client
	UploadSigner *minio.Client
	TaskClient   *asynq.Client
	ChatModel    platformai.ChatModel
	Embedding    platformai.EmbeddingModel
}

func NewServiceContext(ctx context.Context, c config.Config) (*ServiceContext, error) {
	pool, err := database.New(ctx, c.Runtime.Database)
	if err != nil {
		return nil, err
	}
	store, err := objectstore.New(c.Runtime.ObjectStore)
	if err != nil {
		pool.Close()
		return nil, err
	}
	signerSettings := c.Runtime.ObjectStore
	if signerSettings.PublicEndpoint != "" {
		signerSettings.Endpoint = signerSettings.PublicEndpoint
	}
	uploadSigner, err := objectstore.New(signerSettings)
	if err != nil {
		pool.Close()
		return nil, err
	}
	var chatModel platformai.ChatModel
	redisClient := cache.NewRedis(c.Runtime.Redis)
	chatModel, err = airuntime.NewChatModel(ctx, c.Runtime, pool, redisClient)
	if err != nil {
		pool.Close()
		return nil, err
	}
	embedding, err := airuntime.NewEmbeddingModel(c.Runtime)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &ServiceContext{
		Config:       c,
		Database:     pool,
		Redis:        redisClient,
		ObjectStore:  store,
		UploadSigner: uploadSigner,
		TaskClient:   asynq.NewClient(asynq.RedisClientOpt{Addr: c.Runtime.Redis.Addr, Password: c.Runtime.Redis.Password, DB: c.Runtime.Redis.DB}),
		ChatModel:    chatModel,
		Embedding:    embedding,
	}, nil
}

func (s *ServiceContext) Close() error {
	s.Database.Close()
	_ = s.TaskClient.Close()
	return s.Redis.Close()
}
