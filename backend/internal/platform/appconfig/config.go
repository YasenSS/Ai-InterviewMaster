package appconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	Environment string
	ServiceName string
	Database    Database
	Redis       Redis
	ObjectStore ObjectStore
	Parser      Parser
	Security    Security
	Telemetry   Telemetry
	HTTP        HTTP
}

type Database struct {
	URL             string
	MaxConnections  int32
	MinConnections  int32
	MaxConnLifetime time.Duration
}

type Redis struct {
	Addr     string
	Password string
	DB       int
}

type ObjectStore struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

type Parser struct {
	TikaURL string
	ASRURL  string
}

type Security struct {
	JWTSigningKey string
}

type Telemetry struct {
	Enabled  bool
	Endpoint string
	Insecure bool
}

type HTTP struct {
	AllowedOrigins []string
}

// ApplyEnv overlays secret and environment-specific values without ever logging them.
func ApplyEnv(s *Settings) error {
	overrideString("IM_ENV", &s.Environment)
	overrideString("IM_DATABASE_URL", &s.Database.URL)
	overrideString("IM_REDIS_ADDR", &s.Redis.Addr)
	overrideString("IM_REDIS_PASSWORD", &s.Redis.Password)
	overrideString("IM_S3_ENDPOINT", &s.ObjectStore.Endpoint)
	overrideString("IM_S3_REGION", &s.ObjectStore.Region)
	overrideString("IM_S3_BUCKET", &s.ObjectStore.Bucket)
	overrideString("IM_S3_ACCESS_KEY", &s.ObjectStore.AccessKey)
	overrideString("IM_S3_SECRET_KEY", &s.ObjectStore.SecretKey)
	overrideString("IM_TIKA_URL", &s.Parser.TikaURL)
	overrideString("IM_ASR_URL", &s.Parser.ASRURL)
	overrideString("IM_JWT_SIGNING_KEY", &s.Security.JWTSigningKey)
	overrideString("IM_OTEL_ENDPOINT", &s.Telemetry.Endpoint)

	if err := overrideInt("IM_REDIS_DB", &s.Redis.DB); err != nil {
		return err
	}
	if err := overrideBool("IM_S3_USE_PATH_STYLE", &s.ObjectStore.UsePathStyle); err != nil {
		return err
	}
	if err := overrideBool("IM_OTEL_ENABLED", &s.Telemetry.Enabled); err != nil {
		return err
	}
	if err := overrideBool("IM_OTEL_INSECURE", &s.Telemetry.Insecure); err != nil {
		return err
	}

	if value, ok := os.LookupEnv("IM_WEB_ORIGIN"); ok && strings.TrimSpace(value) != "" {
		s.HTTP.AllowedOrigins = []string{strings.TrimSpace(value)}
	}

	return s.Validate()
}

func (s Settings) Validate() error {
	if strings.TrimSpace(s.ServiceName) == "" {
		return fmt.Errorf("runtime service name is required")
	}
	if strings.TrimSpace(s.Database.URL) == "" {
		return fmt.Errorf("database URL is required")
	}
	if strings.TrimSpace(s.Redis.Addr) == "" {
		return fmt.Errorf("redis address is required")
	}
	if s.Database.MaxConnections < 1 {
		return fmt.Errorf("database max connections must be positive")
	}
	if s.Database.MinConnections < 0 || s.Database.MinConnections > s.Database.MaxConnections {
		return fmt.Errorf("database min connections must be between zero and max connections")
	}
	if strings.EqualFold(s.Environment, "production") && len(s.Security.JWTSigningKey) < 32 {
		return fmt.Errorf("production JWT signing key must contain at least 32 characters")
	}
	if s.Telemetry.Enabled && strings.TrimSpace(s.Telemetry.Endpoint) == "" {
		return fmt.Errorf("OpenTelemetry endpoint is required when telemetry is enabled")
	}
	return nil
}

func overrideString(name string, target *string) {
	if value, ok := os.LookupEnv(name); ok {
		*target = strings.TrimSpace(value)
	}
}

func overrideBool(name string, target *bool) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	*target = parsed
	return nil
}

func overrideInt(name string, target *int) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	*target = parsed
	return nil
}

func IntFromEnv(name string, fallback int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
