package appconfig

import "testing"

func TestApplyEnvOverridesValues(t *testing.T) {
	t.Setenv("IM_DATABASE_URL", "postgres://test")
	t.Setenv("IM_REDIS_ADDR", "redis:6379")
	t.Setenv("IM_REDIS_DB", "2")
	t.Setenv("IM_OTEL_ENABLED", "true")
	t.Setenv("IM_OTEL_ENDPOINT", "collector:4317")

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
	if !settings.Telemetry.Enabled {
		t.Fatal("telemetry should be enabled")
	}
}

func TestProductionRequiresStrongJWTKey(t *testing.T) {
	settings := Settings{
		Environment: "production",
		ServiceName: "test",
		Database: Database{URL: "postgres://test", MaxConnections: 2},
		Redis: Redis{Addr: "redis:6379"},
		Security: Security{JWTSigningKey: "short"},
	}
	if err := settings.Validate(); err == nil {
		t.Fatal("Validate() expected an error")
	}
}
