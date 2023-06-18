package context

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/chrisport/go-lang-detector/langdet"
	"github.com/chrisport/go-lang-detector/langdet/langdetdef"
	openai "github.com/sashabaranov/go-openai"
)

const (
	DefaultMaxTokens   = 200
	DefaultN           = 1
	DefaultTemperature = 0.8
	maxRetries         = 3
	cacheLifeTime      = 24 * time.Hour
	N                  = 4
	DefaultPromptFile  = "pkg/openaiclient/context/config/prompt.txt"
	GenerateResponse   = "Generating AI response for question: '%s', asked by user: '%s'"
	// Error messages
	ErrEmptyAPIKey        = "OpenAi API Key is empty"
	ErrFailedPromptPath   = "failed to get absolute path for prompt file: %w"
	ErrFailedPromptRead   = "failed to read prompt file: %w"
	ErrEmptyInput         = "input is empty"
	ErrUninitOpenAI       = "OpenAI client is not initialized"
	ErrFailedChatComplete = "Failed to create chat completion: "
	ErrNoChoicesResponse  = "no choices in response"
	ErrNonEnglishInput    = "input is not in English"
	ErrFailedModeration   = "failed to moderate text: "
	ErrNoModResults       = "no choices were returned in the moderation response"
	ErrMaxRetries         = "failed to moderate text after maximum retries"
)

type CacheItem struct {
	Conversation []openai.ChatCompletionMessage
	Timestamp    time.Time
}

type OpenAiContext struct {
	APIKey  string
	Prompt  string
	Client  *openai.Client
	workers int
	sem     chan struct{}

	generationCache map[string]CacheItem

	generationCacheMu sync.RWMutex

	cacheLifeTime time.Duration
}

func NewOpenAiContext(apiKey string, workers int) (*OpenAiContext, error) {
	if apiKey == "" {
		return nil, fmt.Errorf(ErrEmptyAPIKey)
	}

	prompt, err := getPrompt()
	if err != nil {
		return nil, err
	}

	client := &OpenAiContext{
		APIKey:          apiKey,
		Prompt:          prompt,
		Client:          openai.NewClient(apiKey),
		workers:         getWorkerCount(workers),
		sem:             make(chan struct{}, workers),
		cacheLifeTime:   cacheLifeTime,
		generationCache: make(map[string]CacheItem),
	}

	go client.RunCacheEviction()

	return client, nil
}

func getWorkerCount(workers int) int {
	if workers <= 0 {
		return 1
	}
	return workers
}

func getPrompt() (string, error) {
	promptPath, err := filepath.Abs(DefaultPromptFile)
	if err != nil {
		return "", fmt.Errorf(ErrFailedPromptPath, err)
	}

	prompt, err := ioutil.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf(ErrFailedPromptRead, err)
	}

	return string(prompt), nil
}

func (client *OpenAiContext) GenerateResponse(input string, authorUsername string) (string, error) {
	log.Printf(GenerateResponse, input, authorUsername)
	if client.Client == nil {
		return "", fmt.Errorf(ErrUninitOpenAI)
	}

	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf(ErrEmptyInput)
	}

	cacheKey := authorUsername
	client.generationCacheMu.RLock()
	cacheItem, ok := client.generationCache[cacheKey]
	client.generationCacheMu.RUnlock()

	var conversation []openai.ChatCompletionMessage

	systemMessage := fmt.Sprintf(client.Prompt, authorUsername)
	systemMessageExists := false

	if ok {
		conversation = append(conversation, cacheItem.Conversation...)
		for _, message := range cacheItem.Conversation {
			if message.Role == openai.ChatMessageRoleAssistant && message.Content == systemMessage {
				systemMessageExists = true
				break
			}
		}
	}

	if !systemMessageExists {
		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: systemMessage,
		})
	}

	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: input,
	})

	if len(conversation) > N {
		conversation = conversation[len(conversation)-N:]
	}

	ctx := context.Background()

	req := client.createChatCompletionRequest(conversation)

	response, err := client.performChatCompletion(ctx, req)

	if err != nil {
		return "", err
	}

	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: response,
	})

	client.generationCacheMu.Lock()
	client.generationCache[cacheKey] = CacheItem{
		Conversation: conversation,
		Timestamp:    time.Now(),
	}
	client.generationCacheMu.Unlock()

	return response, nil
}

func (client *OpenAiContext) createChatCompletionRequest(conversation []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo16K,
		Messages:    conversation,
		MaxTokens:   DefaultMaxTokens,
		N:           DefaultN,
		Temperature: DefaultTemperature,
	}
}

func retryWithBackoff(performFunc func() (interface{}, error), maxRetries int) (interface{}, error) {
	bo := backoff.NewExponentialBackOff()
	retryCount := 0
	var result interface{}
	var err error
	for {
		if result, err = performFunc(); err != nil {
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

func (client *OpenAiContext) performChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	var allResponses strings.Builder

	for {
		respInterface, err := retryWithBackoff(func() (interface{}, error) {
			client.sem <- struct{}{}        // Acquire semaphore
			defer func() { <-client.sem }() // Ensure semaphore release
			return client.Client.CreateChatCompletion(ctx, req)
		}, maxRetries)
		if err != nil {
			return "", fmt.Errorf("%s %v", ErrFailedChatComplete, err)
		}

		response, ok := respInterface.(openai.ChatCompletionResponse)
		if !ok {
			return "", errors.New("failed to cast to openai.ChatCompletionResponse")
		}

		if len(response.Choices) == 0 {
			return "", fmt.Errorf(ErrNoChoicesResponse)
		}

		responseText := response.Choices[0].Message.Content
		finishReason := response.Choices[0].FinishReason

		allResponses.WriteString(responseText)

		if finishReason == openai.FinishReasonStop {
			return allResponses.String(), nil
		} else {
			// Otherwise, generate another response
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: responseText,
			})

			continue
		}
	}
}

func checkLanguage(input string) error {
	detector := langdet.NewDetector()
	detector.AddLanguageComparators(langdetdef.ENGLISH)

	detectedLanguage := detector.GetClosestLanguage(input)
	if detectedLanguage != "english" {
		return fmt.Errorf("%s, detected language is: %s", ErrNonEnglishInput, detectedLanguage)
	}

	return nil
}

func (client *OpenAiContext) ModerationCheck(input string, maxRetries int) (bool, error) {
	if client.Client == nil {
		return false, fmt.Errorf(ErrUninitOpenAI)
	}

	if strings.TrimSpace(input) == "" {
		return false, fmt.Errorf(ErrEmptyInput)
	}

	if err := checkLanguage(input); err != nil {
		return false, err
	}

	ctx := context.Background()

	req := client.createModerationRequest(input)

	result, err := client.performModeration(ctx, req, maxRetries)

	if err != nil {
		return false, err
	}

	return result, nil
}

func (client *OpenAiContext) createModerationRequest(input string) openai.ModerationRequest {
	return openai.ModerationRequest{
		Model: openai.ModerationTextLatest,
		Input: input,
	}
}

func (client *OpenAiContext) performModeration(ctx context.Context, req openai.ModerationRequest, maxRetries int) (bool, error) {
	bo := backoff.NewExponentialBackOff()
	retryCount := 0

	for {
		if retryCount >= maxRetries {
			return false, fmt.Errorf(ErrMaxRetries)
		}

		client.sem <- struct{}{}
		resp, err := client.Client.Moderations(ctx, req)
		<-client.sem

		if err != nil {
			nextInterval := bo.NextBackOff()
			if nextInterval != backoff.Stop {
				retryCount++
				continue
			}
			return false, fmt.Errorf("%s %v", ErrFailedModeration, err)
		}

		if len(resp.Results) == 0 {
			return false, fmt.Errorf(ErrNoModResults)
		}

		return resp.Results[0].Flagged, nil
	}
}

func (client *OpenAiContext) Close() {
	close(client.sem)
}

func (client *OpenAiContext) RunCacheEviction() {
	ticker := time.NewTicker(client.cacheLifeTime)

	for {
		<-ticker.C
		client.generationCacheMu.Lock()
		for k, v := range client.generationCache {
			if time.Since(v.Timestamp) > client.cacheLifeTime {
				delete(client.generationCache, k)
			}
		}
		client.generationCacheMu.Unlock()
	}
}
