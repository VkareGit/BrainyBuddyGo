package context

import (
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
)

type OpenAiContext struct {
	Client          *openai.Client
	Config          *OpenAiContextConfig
	sem             chan struct{}
	generationCache sync.Map
}

func NewOpenAiContext(apiKey string, workers int, promptData []byte, production bool) (*OpenAiContext, error) {
	if apiKey == "" {
		return nil, ErrEmptyAPIKey
	}

	client := openai.NewClient(apiKey)

	prompt, err := getPrompt(promptData, production)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	config := &OpenAiContextConfig{
		APIKey:                apiKey,
		Workers:               workers,
		CacheLifeTime:         cacheLifeTime,
		ConversationCacheSize: ConversationCacheSize,
		DefaultMaxTokens:      DefaultMaxTokens,
		DefaultN:              DefaultN,
		DefaultTemperature:    DefaultTemperature,
		MaxRetries:            maxRetries,
		DefaultPromptFile:     prompt,
	}

	ctx := &OpenAiContext{
		Client:          client,
		Config:          config,
		sem:             make(chan struct{}, getWorkerCount(workers)),
		generationCache: sync.Map{},
	}

	go ctx.RunCacheEviction()

	return ctx, nil
}

func (client *OpenAiContext) Close() {
	close(client.sem)
}
