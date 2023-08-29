package openai

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sashabaranov/go-openai"
)

const (
	DefaultMaxTokens        = 4096
	DefaultTemperature      = 0.3
	EstimatedResponseTokens = 100
	MaxConcurrentRequests   = 10 // OpenAI API allows 10 concurrent requests
	MaxRetryAttempts        = 3  // Maximum number of retries
)

var (
	//go:embed prompt.json
	promptData []byte
)

type OpenAiContext struct {
	Client *openai.Client
	sem    chan struct{}
}

func NewClient(apiKey string) (*OpenAiContext, error) {
	if apiKey == "" {
		return nil, errors.New("API key is required")
	}

	openAiContext := &OpenAiContext{
		Client: openai.NewClient(apiKey),
		sem:    make(chan struct{}, MaxConcurrentRequests),
	}

	return openAiContext, nil
}

func (c *OpenAiContext) GenerateAnswer(ctx context.Context, input string) (string, error) {
	if c.Client == nil {
		return "", errors.New("OpenAI client is not initialized")
	}

	if strings.TrimSpace(input) == "" {
		return "", errors.New("Prompt is required")
	}

	tokenCount := EstimateTokenCount(input) + EstimatedResponseTokens
	if tokenCount > DefaultMaxTokens {
		return "", fmt.Errorf("Input is too long (%d tokens). Maximum allowed is %d tokens", tokenCount, DefaultMaxTokens)
	}

	prompt, err := getPrompt(promptData)
	if err != nil {
		return "", err
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: prompt},
		{Role: openai.ChatMessageRoleUser, Content: input},
	}

	conversation := c.createChatCompletionRequest(messages)

	response, err := c.performChatCompletion(ctx, conversation)
	if err != nil {
		return "", fmt.Errorf("Chat completion failed: %w", err)
	}

	response = strings.TrimSpace(response)

	return response, nil
}

func (c *OpenAiContext) createChatCompletionRequest(messages []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:       openai.GPT40613,
		Messages:    messages,
		MaxTokens:   DefaultMaxTokens,
		Temperature: DefaultTemperature,
	}
}

func (c *OpenAiContext) performChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	respInterface, err := c.retryOperationWithExponentialBackoff(ctx, func() (interface{}, error) {
		return c.Client.CreateChatCompletion(ctx, req)
	}, MaxRetryAttempts)
	if err != nil {
		return "", err
	}

	response, ok := respInterface.(openai.ChatCompletionResponse)
	if !ok {
		return "", errors.New("failed to cast to openai.ChatCompletionResponse")
	}

	if response.Choices == nil || len(response.Choices) == 0 {
		return "", errors.New("The response is empty")
	}

	if response.Choices[0].FinishReason != openai.FinishReasonStop {
		return "", errors.New("The response is incomplete")
	}

	return response.Choices[0].Message.Content, nil
}

func EstimateTokenCount(s string) int {
	return len([]byte(s)) / 4
}

func (c *OpenAiContext) AnswerQuestion(ctx context.Context, question string) (string, error) {
	answer, err := c.GenerateAnswer(ctx, question)
	if err != nil {
		return "", err
	}

	return answer, nil
}

func (c *OpenAiContext) retryOperationWithExponentialBackoff(ctx context.Context, performFunc func() (interface{}, error), maxRetries int) (interface{}, error) {
	bo := backoff.NewExponentialBackOff()
	retryCount := 0
	var result interface{}
	var err error
	for {
		c.sem <- struct{}{} // Acquire semaphore
		result, err = performFunc()
		<-c.sem // Ensure semaphore release
		if err != nil {
			if retryCount >= maxRetries {
				return nil, fmt.Errorf("%w after maximum retries", err)
			}
			nextInterval := bo.NextBackOff()
			if nextInterval != backoff.Stop {
				retryCount++
				time.Sleep(nextInterval)
				continue
			}
			return nil, err
		}
		return result, nil
	}
}

func getPrompt(promptData []byte) (string, error) {

	var allPrompts []string

	var config map[string]map[string][]string
	err := json.Unmarshal(promptData, &config)
	if err != nil {
		return "", fmt.Errorf("failed to decode config file: %w", err)
	}

	teamAdvisorConfig, ok := config["team-advisor"]
	if !ok {
		return "", fmt.Errorf("team-advisor config not found")
	}

	for _, prompts := range teamAdvisorConfig {
		allPrompts = append(allPrompts, strings.Join(prompts, " "))
	}

	return strings.Join(allPrompts, " "), nil
}
