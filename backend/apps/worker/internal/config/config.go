package config

import (
	"time"

	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
)

type Config struct {
	Runtime         appconfig.Settings
	Concurrency     int
	ShutdownTimeout time.Duration
	Queues          map[string]int
}
