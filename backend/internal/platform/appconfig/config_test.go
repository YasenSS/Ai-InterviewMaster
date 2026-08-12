package appconfig

import (
	"testing"
	"time"
)

func TestApplyEnvOverridesValues(t *testing.T) {
	t.Setenv("IM_DATABASE_URL", "postgres://test")
	t.Setenv("IM_REDIS_ADDR", "redis:6379")
	t.Setenv("IM_REDIS_DB", "2")
	t.Setenv("IM_S3_PUBLIC_ENDPOINT", "https://objects.example.com")
	t.Setenv("IM_OTEL_ENABLED", "true")
	t.Setenv("IM_OTEL_ENDPOINT", "collector:4317")
	t.Setenv("IM_AI_ENABLED", "true")
	t.Setenv("IM_AI_PROVIDER", "openai-compatible")
	t.Setenv("IM_AI_API_KEY", "test-key")
	t.Setenv("IM_AI_CHAT_MODEL", "test-model")
	t.Setenv("IM_AI_REQUEST_TIMEOUT", "45s")
	t.Setenv("IM_AI_MAX_OUTPUT_TOKENS", "2048")

	settings := Settings{
		ServiceName: "test",
		Database: Database{
			MaxConnections: 2,
		},
	}
	if err := ApplyEnv(&settings); err != nil {
		t.Fatalf("ApplyEnv() error = %v", err)
	}
	if settings.Database.URL != "postgres://test" {
		t.Fatalf("database URL = %q", settings.Database.URL)
	}
	if settings.Redis.DB != 2 {
		t.Fatalf("redis DB = %d", settings.Redis.DB)
	}
	if settings.ObjectStore.PublicEndpoint != "https://objects.example.com" {
		t.Fatalf("public object endpoint = %q", settings.ObjectStore.PublicEndpoint)
	}
	if !settings.Telemetry.Enabled {
		t.Fatal("telemetry should be enabled")
	}
	if !settings.AI.Enabled || settings.AI.RequestTimeout != 45*time.Second || settings.AI.MaxOutputTokens != 2048 {
		t.Fatalf("AI settings = %#v", settings.AI)
	}
}

func TestProductionRequiresStrongJWTKey(t *testing.T) {
	settings := Settings{
		Environment: "production",
		ServiceName: "test",
		Database:    Database{URL: "postgres://test", MaxConnections: 2},
		Redis:       Redis{Addr: "redis:6379"},
		Security:    Security{JWTSigningKey: "short"},
	}
	if err := settings.Validate(); err == nil {
		t.Fatal("Validate() expected an error")
	}
}

func TestEnabledAIRequiresCredentialsAndLimits(t *testing.T) {
	settings := Settings{
		ServiceName: "test",
		Database:    Database{URL: "postgres://test", MaxConnections: 2},
		Redis:       Redis{Addr: "redis:6379"},
		AI:          AI{Enabled: true, Provider: "openai-compatible"},
	}
	if err := settings.Validate(); err == nil {
		t.Fatal("Validate() expected an AI configuration error")
	}
}
