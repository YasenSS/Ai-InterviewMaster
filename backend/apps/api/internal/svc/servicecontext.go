// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/config"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/cache"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/database"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/objectstore"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type ServiceContext struct {
	Config   config.Config
	Database *pgxpool.Pool
	Redis    *redis.Client
	ObjectStore *minio.Client
	TaskClient *asynq.Client
}

func NewServiceContext(ctx context.Context, c config.Config) (*ServiceContext, error) {
	pool, err := database.New(ctx, c.Runtime.Database)
	if err != nil {
		return nil, err
	}
	store, err := objectstore.New(c.Runtime.ObjectStore)
	if err != nil { pool.Close(); return nil, err }
	return &ServiceContext{
		Config:   c,
		Database: pool,
		Redis:    cache.NewRedis(c.Runtime.Redis),
		ObjectStore: store,
		TaskClient: asynq.NewClient(asynq.RedisClientOpt{Addr:c.Runtime.Redis.Addr, Password:c.Runtime.Redis.Password, DB:c.Runtime.Redis.DB}),
	}, nil
}

func (s *ServiceContext) Close() error {
	s.Database.Close()
	_ = s.TaskClient.Close()
	return s.Redis.Close()
}
