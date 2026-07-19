// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/config"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/handler"
	"github.com/interviewmaster/interviewmaster/backend/apps/api/internal/svc"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/apperror"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/logging"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/requestid"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/telemetry"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/interviewmaster.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	port, err := appconfig.IntFromEnv("IM_API_PORT", c.Port)
	if err != nil {
		panic(err)
	}
	c.Port = port
	if err := appconfig.ApplyEnv(&c.Runtime); err != nil {
		panic(err)
	}
	c.Auth.AccessSecret = c.Runtime.Security.JWTSigningKey
	logging.Setup(c.Log.Level)

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

	httpx.SetErrorHandlerCtx(apperror.HTTPResponse)

	server := rest.MustNewServer(c.RestConf, rest.WithCors(c.Runtime.HTTP.AllowedOrigins...))
	defer server.Stop()
	server.Use(requestid.Middleware)
	server.Use(telemetry.HTTPMiddleware(c.Runtime.ServiceName))

	serviceContext, err := svc.NewServiceContext(ctx, c)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := serviceContext.Close(); err != nil {
			slog.Error("service context shutdown failed", "error", err)
		}
	}()
	handler.RegisterHandlers(server, serviceContext)

	slog.Info("starting API server", "host", c.Host, "port", c.Port, "environment", c.Runtime.Environment)
	server.Start()
}
