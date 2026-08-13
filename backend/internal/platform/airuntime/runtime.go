package airuntime

import (
	"context"
	"strings"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/ai/provider"
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// NewChatModel builds the instrumented provider stack used by API and Worker.
func NewChatModel(ctx context.Context, settings appconfig.Settings, db *pgxpool.Pool, redisClient *redis.Client) (platformai.ChatModel, error) {
	if !settings.AI.Enabled {
		return nil, nil
	}
	primary, err := provider.NewOpenAI(ctx, provider.OpenAIConfig{
		Provider:          settings.AI.Provider,
		BaseURL:           settings.AI.BaseURL,
		APIKey:            settings.AI.APIKey,
		Model:             settings.AI.ChatModel,
		Timeout:           settings.AI.RequestTimeout,
		MaxOutputTokens:   settings.AI.MaxOutputTokens,
		StructuredOutputs: settings.AI.StructuredOutputs,
	})
	if err != nil {
		return nil, err
	}
	small := platformai.ChatModel(primary)
	if strings.TrimSpace(settings.AI.SmallModel) != "" && settings.AI.SmallModel != settings.AI.ChatModel {
		smallModel, smallErr := provider.NewOpenAI(ctx, provider.OpenAIConfig{
			Provider:          settings.AI.Provider,
			BaseURL:           settings.AI.BaseURL,
			APIKey:            settings.AI.APIKey,
			Model:             settings.AI.SmallModel,
			Timeout:           settings.AI.RequestTimeout,
			MaxOutputTokens:   settings.AI.MaxOutputTokens,
			StructuredOutputs: settings.AI.StructuredOutputs,
		})
		if smallErr != nil {
			return nil, smallErr
		}
		small = smallModel
	}
	quota := platformai.DefaultQuota()
	if settings.AI.MaxInputRunes > 0 {
		quota.MaxInputRunes = settings.AI.MaxInputRunes
	}
	if settings.AI.MaxInflight > 0 {
		quota.MaxInflight = settings.AI.MaxInflight
	}
	if settings.AI.DailyCallsHard > 0 {
		quota.DailyCallsHard = settings.AI.DailyCallsHard
	}
	if settings.AI.DailyCallsSoft > 0 {
		quota.DailyCallsSoft = settings.AI.DailyCallsSoft
	}
	var limiter *platformai.Limiter
	if redisClient != nil {
		limiter = platformai.NewLimiter(platformai.RedisCounters{Client: redisClient}, quota)
	} else {
		limiter = platformai.NewLimiter(platformai.NewMemoryCounters(), quota)
	}
	return platformai.NewInstrumented(
		primary,
		small,
		platformai.PostgresInvocations{DB: db},
		limiter,
		platformai.Pricing{
			PromptMicrosPer1k:     settings.AI.PromptMicrosPer1k,
			CompletionMicrosPer1k: settings.AI.CompletionMicrosPer1k,
		},
		strings.EqualFold(settings.Environment, "production"),
	), nil
}

// NewEmbeddingModel returns an OpenAI-compatible embedding client when configured.
func NewEmbeddingModel(settings appconfig.Settings) (platformai.EmbeddingModel, error) {
	if !settings.AI.Enabled || strings.TrimSpace(settings.AI.EmbeddingModel) == "" {
		return nil, nil
	}
	return provider.NewOpenAIEmbedding(provider.EmbeddingConfig{
		Provider: settings.AI.Provider,
		BaseURL:  settings.AI.BaseURL,
		APIKey:   settings.AI.APIKey,
		Model:    settings.AI.EmbeddingModel,
		Timeout:  settings.AI.RequestTimeout,
	})
}
