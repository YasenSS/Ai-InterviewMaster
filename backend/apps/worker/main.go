package main

import (
	"context"
	"flag"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	workerconfig "github.com/interviewmaster/interviewmaster/backend/apps/worker/internal/config"
	"github.com/interviewmaster/interviewmaster/backend/apps/worker/internal/tasks"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/airuntime"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/cache"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/database"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/logging"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/objectstore"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/telemetry"
	sharedtasks "github.com/interviewmaster/interviewmaster/backend/internal/tasks"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/conf"
)

var configFile = flag.String("f", "etc/worker.yaml", "the config file")

func main() {
	flag.Parse()

	var c workerconfig.Config
	conf.MustLoad(*configFile, &c)
	if err := appconfig.ApplyEnv(&c.Runtime); err != nil {
		panic(err)
	}
	db, err := database.New(context.Background(), c.Runtime.Database)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	store, err := objectstore.New(c.Runtime.ObjectStore)
	if err != nil {
		panic(err)
	}
	logging.Setup("info")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	shutdownTelemetry, err := telemetry.Setup(ctx, c.Runtime)
	if err != nil {
		panic(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			slog.Error("telemetry shutdown failed", "error", err)
		}
	}()

	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     c.Runtime.Redis.Addr,
			Password: c.Runtime.Redis.Password,
			DB:       c.Runtime.Redis.DB,
		},
		asynq.Config{
			Concurrency:     c.Concurrency,
			Queues:          c.Queues,
			ShutdownTimeout: c.ShutdownTimeout,
		},
	)

	redisClient := cache.NewRedis(c.Runtime.Redis)
	defer redisClient.Close()
	chatModel, err := airuntime.NewChatModel(context.Background(), c.Runtime, db, redisClient)
	if err != nil {
		panic(err)
	}

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeNoop, tasks.HandleNoop)
	mux.Handle(tasks.TypeResumeParse, tasks.ResumeParseHandler(db, store, c.Runtime.ObjectStore.Bucket, c.Runtime.Parser.TikaURL, chatModel))
	mux.Handle(sharedtasks.TypeQuestionGenerate, tasks.QuestionGenerateHandler(db, chatModel))
	mux.Handle(sharedtasks.TypeReportGenerate, tasks.ReportGenerateHandler(db, chatModel))
	mux.Handle("object:cleanup", tasks.ObjectCleanupHandler(db, store, c.Runtime.ObjectStore.Bucket))
	mux.Handle("asr:transcribe", tasks.ASRHandler(db, store, c.Runtime.ObjectStore.Bucket, c.Runtime.Parser.ASRURL))

	slog.Info("starting worker", "environment", c.Runtime.Environment, "concurrency", c.Concurrency)
	go runOpsLoop(ctx, db)
	if err := server.Run(mux); err != nil {
		panic(err)
	}
}
