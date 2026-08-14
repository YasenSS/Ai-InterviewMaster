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
	AI          AI
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
	Endpoint       string
	PublicEndpoint string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UsePathStyle   bool
}

type Parser struct {
	TikaURL string
	ASRURL  string
}

// AI contains provider-neutral model settings. Provider-specific SDK types
// deliberately stay in internal/platform/ai/provider.
type AI struct {
	Enabled               bool
	Provider              string
	BaseURL               string
	APIKey                string
	ChatModel             string
	SmallModel            string
	EmbeddingModel        string
	RequestTimeout        time.Duration
	MaxOutputTokens       int
	StructuredOutputs     bool
	MaxInputRunes         int
	MaxInflight           int
	DailyCallsSoft        int
	DailyCallsHard        int
	PromptMicrosPer1k     int64 `json:",default=0"`
	CompletionMicrosPer1k int64 `json:",default=0"`
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
	overrideString("IM_S3_PUBLIC_ENDPOINT", &s.ObjectStore.PublicEndpoint)
	overrideString("IM_S3_REGION", &s.ObjectStore.Region)
	overrideString("IM_S3_BUCKET", &s.ObjectStore.Bucket)
	overrideString("IM_S3_ACCESS_KEY", &s.ObjectStore.AccessKey)
	overrideString("IM_S3_SECRET_KEY", &s.ObjectStore.SecretKey)
	overrideString("IM_TIKA_URL", &s.Parser.TikaURL)
	overrideString("IM_ASR_URL", &s.Parser.ASRURL)
	overrideString("IM_AI_PROVIDER", &s.AI.Provider)
	overrideString("IM_AI_BASE_URL", &s.AI.BaseURL)
	overrideString("IM_AI_API_KEY", &s.AI.APIKey)
	overrideString("IM_AI_CHAT_MODEL", &s.AI.ChatModel)
	overrideString("IM_AI_SMALL_MODEL", &s.AI.SmallModel)
	overrideString("IM_AI_EMBEDDING_MODEL", &s.AI.EmbeddingModel)
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
	if err := overrideBool("IM_AI_ENABLED", &s.AI.Enabled); err != nil {
		return err
	}
	if err := overrideBool("IM_AI_STRUCTURED_OUTPUTS", &s.AI.StructuredOutputs); err != nil {
		return err
	}
	if err := overrideInt("IM_AI_MAX_OUTPUT_TOKENS", &s.AI.MaxOutputTokens); err != nil {
		return err
	}
	if err := overrideDuration("IM_AI_REQUEST_TIMEOUT", &s.AI.RequestTimeout); err != nil {
		return err
	}
	if err := overrideInt("IM_AI_MAX_INPUT_RUNES", &s.AI.MaxInputRunes); err != nil {
		return err
	}
	if err := overrideInt("IM_AI_MAX_INFLIGHT", &s.AI.MaxInflight); err != nil {
		return err
	}
	if err := overrideInt("IM_AI_DAILY_CALLS_SOFT", &s.AI.DailyCallsSoft); err != nil {
		return err
	}
	if err := overrideInt("IM_AI_DAILY_CALLS_HARD", &s.AI.DailyCallsHard); err != nil {
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
	if s.AI.Enabled {
		if !strings.EqualFold(strings.TrimSpace(s.AI.Provider), "openai") &&
			!strings.EqualFold(strings.TrimSpace(s.AI.Provider), "openai-compatible") {
			return fmt.Errorf("AI provider must be openai or openai-compatible")
		}
		if strings.TrimSpace(s.AI.APIKey) == "" {
			return fmt.Errorf("AI API key is required when AI is enabled")
		}
		if strings.TrimSpace(s.AI.ChatModel) == "" {
			return fmt.Errorf("AI chat model is required when AI is enabled")
		}
		if s.AI.RequestTimeout <= 0 {
			return fmt.Errorf("AI request timeout must be positive when AI is enabled")
		}
		if s.AI.MaxOutputTokens < 1 {
			return fmt.Errorf("AI max output tokens must be positive when AI is enabled")
		}
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

func overrideDuration(name string, target *time.Duration) error {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.ParseDuration(value)
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
