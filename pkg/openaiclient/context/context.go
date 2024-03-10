package openai

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sashabaranov/go-openai"
)

const (
	DefaultMaxTokens        = 4096
	DefaultTemperature      = 0.1
	EstimatedResponseTokens = 100
	MaxConcurrentRequests   = 10
	MaxRetryAttempts        = 2
)

var (
	//go:embed prompt.json
	promptData []byte

	prompt string
)

func init() {
	var err error
	prompt, err = getPrompt(promptData)
	if err != nil {
		log.Fatalf("Failed to load prompts: %v", err)
	}
}

type OpenAiContext struct {
	Client *openai.Client
	sem    chan struct{}
	mutex  sync.Mutex
}

func NewClient(apiKey string) (*OpenAiContext, error) {
	if apiKey == "" {
		return nil, errors.New("API key is required")
	}

	return &OpenAiContext{
		Client: openai.NewClient(apiKey),
		sem:    make(chan struct{}, MaxConcurrentRequests),
	}, nil
}

func (c *OpenAiContext) GenerateAnswer(ctx context.Context, input string) (string, error) {
	if c.Client == nil {
		return "", errors.New("OpenAI client is not initialized")
	}

	if strings.TrimSpace(input) == "" {
		return "", errors.New("Prompt is required")
	}

	tokenCount, err := c.estimateTokenCount(input)
	if err != nil {
		return "", err
	}

	if tokenCount > DefaultMaxTokens {
		return "", fmt.Errorf("Input is too long (%d tokens). Maximum allowed is %d tokens", tokenCount, DefaultMaxTokens)
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: prompt},
		{Role: openai.ChatMessageRoleUser, Content: input},
	}

	conversation := c.createChatCompletionRequest(messages)

	c.sem <- struct{}{} // Acquire a semaphore slot after all checks
	defer func() {
		<-c.sem // Release the semaphore slot
	}()

	response, err := c.performChatCompletionWithRetries(ctx, conversation, MaxRetryAttempts)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

func (c *OpenAiContext) createChatCompletionRequest(messages []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:       openai.GPT40613,
		Messages:    messages,
		MaxTokens:   DefaultMaxTokens,
		Temperature: DefaultTemperature,
	}
}

func (c *OpenAiContext) performChatCompletionWithRetries(ctx context.Context, req openai.ChatCompletionRequest, retries int) (string, error) {
	bo := backoff.NewExponentialBackOff()
	var response openai.ChatCompletionResponse
	var err error

	for attempts := 0; attempts <= retries; attempts++ {
		c.sem <- struct{}{} // Acquire a semaphore slot
		c.mutex.Lock()
		response, err = c.Client.CreateChatCompletion(ctx, req)
		c.mutex.Unlock()
		<-c.sem // Release the semaphore slot

		if err == nil {
			return response.Choices[0].Message.Content, nil
		}

		if strings.Contains(err.Error(), "429") {
			retryDuration := extractRetryDuration(err.Error())
			log.Printf("Rate limit error, waiting for %v before retrying...", retryDuration)
			time.Sleep(retryDuration)
		} else {
			time.Sleep(bo.NextBackOff())
		}
	}
	return "", err
}

const MinimumBackoff = 500 * time.Millisecond // 500ms

func extractRetryDuration(errMsg string) time.Duration {
	re := regexp.MustCompile(`try again in (\d+)ms`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) < 2 {
		return MinimumBackoff
	}
	ms, _ := strconv.Atoi(matches[1])
	duration := time.Millisecond * time.Duration(ms)
	if duration < MinimumBackoff {
		duration = MinimumBackoff
	}
	return duration
}

func (c *OpenAiContext) estimateTokenCount(s string) (int, error) {
	// A rough estimation based on the information given
	return len(s) / 4, nil
}

type Config struct {
	DiscordLeagueSupport struct {
		Introduction          []string     `json:"introduction"`
		InteractionGuidelines []string     `json:"interaction_guidelines"`
		ApiRequestInstruction []string     `json:"api_request_instruction"`
		ApiSpecificRequests   []ApiRequest `json:"api_specific_requests"`
		GeneralGameplayAdvice []string     `json:"general_gameplay_advice"`
	} `json:"discord-league-support"`
}

type ApiRequest struct {
	RequestType     string   `json:"request_type"`
	TriggerKeywords []string `json:"trigger_keywords"`
	ResponseFormat  string   `json:"response_format"`
}

func getPrompt(promptData []byte) (string, error) {
	var config Config
	err := json.Unmarshal(promptData, &config)
	if err != nil {
		return "", fmt.Errorf("failed to decode config file: %w", err)
	}

	var allPrompts []string

	// Adding introduction, interaction guidelines and api request instructions to the prompts
	allPrompts = append(allPrompts, config.DiscordLeagueSupport.Introduction...)
	allPrompts = append(allPrompts, config.DiscordLeagueSupport.InteractionGuidelines...)
	allPrompts = append(allPrompts, config.DiscordLeagueSupport.ApiRequestInstruction...)

	// Processing API specific requests
	for _, request := range config.DiscordLeagueSupport.ApiSpecificRequests {
		prompt := fmt.Sprintf("%s: %s", request.RequestType, request.ResponseFormat)
		allPrompts = append(allPrompts, prompt)
	}

	// Adding general gameplay advice to the prompts
	allPrompts = append(allPrompts, config.DiscordLeagueSupport.GeneralGameplayAdvice...)

	return strings.Join(allPrompts, " "), nil
}
