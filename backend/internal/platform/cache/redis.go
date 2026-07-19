package cache

import (
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"github.com/redis/go-redis/v9"
)

func NewRedis(settings appconfig.Redis) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     settings.Addr,
		Password: settings.Password,
		DB:       settings.DB,
	})
}
