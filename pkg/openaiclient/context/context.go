package context

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/chrisport/go-lang-detector/langdet"
	"github.com/chrisport/go-lang-detector/langdet/langdetdef"
	"github.com/sashabaranov/go-openai"
)

const (
	DefaultMaxTokens      = 200
	DefaultN              = 1
	DefaultTemperature    = 0.8
	maxRetries            = 3
	cacheLifeTime         = 24 * time.Hour
	ConversationCacheSize = 2
	DefaultPromptFile     = "pkg/openaiclient/context/config/prompt.json"
	GenerateResponse      = "Generating AI response for question: '%s', asked by user: '%s'"
)

var (
	ErrEmptyAPIKey        = errors.New("OpenAi API Key is empty")
	ErrUninitOpenAI       = errors.New("OpenAI client is not initialized")
	ErrFailedChatComplete = errors.New("failed to create chat completion")
	ErrNoChoicesResponse  = errors.New("no choices in response")
	ErrNonEnglishInput    = errors.New("input is not in English")
	ErrFailedModeration   = errors.New("failed to moderate text")
	ErrNoModResults       = errors.New("no choices were returned in the moderation response")
	ErrMaxRetries         = errors.New("failed to moderate text after maximum retries")
	ErrEmptyInput         = errors.New("input is empty")
)

type TeamAdvisorConfig struct {
	Welcome               []string `json:"welcome"`
	AppInterface          []string `json:"app_interface"`
	AppSettings           []string `json:"app_settings"`
	BanningPhase          []string `json:"banning_phase"`
	PickingPhase          []string `json:"picking_phase"`
	ExampleQuestion       []string `json:"example_question"`
	QuickSettingsFeatures []string `json:"quick_settings_features"`
	AppSettingsDetails    []string `json:"app_settings_details"`
	BanningPhaseDetails   []string `json:"banning_phase_details"`
	PickingPhaseDetails   []string `json:"picking_phase_details"`
	TimeoutHandling       []string `json:"timeout_handling"`
}

type TeamAdvisorData struct {
	TeamAdvisorConfig `json:"team-advisor"`
}

type NormalPromptConfig struct {
	Welcome []string `json:"welcome"`
}

type NormalPromptData struct {
	NormalPromptConfig `json:"normal"`
}

type CacheItem struct {
	Conversation []openai.ChatCompletionMessage
	Timestamp    time.Time
	IsFinished   bool
}

type UserCacheItem struct {
	Conversations []CacheItem
}
type OpenAiContextConfig struct {
	APIKey                string
	Workers               int
	CacheLifeTime         time.Duration
	ConversationCacheSize int
	DefaultMaxTokens      int
	DefaultN              int
	DefaultTemperature    float64
	MaxRetries            int
	DefaultPromptFile     string
}

type OpenAiContext struct {
	Client          *openai.Client
	Config          *OpenAiContextConfig
	sem             chan struct{}
	generationCache sync.Map
}

func NewOpenAiContext(apiKey string, workers int, basepath string, production bool) (*OpenAiContext, error) {
	if apiKey == "" {
		return nil, ErrEmptyAPIKey
	}

	client := openai.NewClient(apiKey)

	prompt, err := getPrompt(basepath, production)
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

func getWorkerCount(workers int) int {
	if workers <= 0 {
		return 1
	}
	return workers
}

func getPrompt(basepath string, production bool) (string, error) {
	filepath := filepath.Join(basepath, DefaultPromptFile)

	promptBytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file: %w", err)
	}

	var allPrompts []string

	if !production {
		var config NormalPromptData
		err = json.Unmarshal(promptBytes, &config)
		if err != nil {
			return "", fmt.Errorf("failed to decode config file: %w", err)
		}
		allPrompts = append(allPrompts, strings.Join(config.Welcome, " "))
	} else {
		var config map[string]map[string][]string
		err = json.Unmarshal(promptBytes, &config)
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
	}

	return strings.Join(allPrompts, " "), nil
}

func (client *OpenAiContext) AddItemToCache(key interface{}, value UserCacheItem) {
	client.generationCache.Store(key, value)
}

func (client *OpenAiContext) DeleteItemFromCache(key interface{}) {
	client.generationCache.Delete(key)
}

func (client *OpenAiContext) CacheContains(key interface{}) (value interface{}, ok bool) {
	value, ok = client.generationCache.Load(key)
	return value, ok
}

func (client *OpenAiContext) GenerateResponse(input string, authorUsername string) (string, error) {
	log.Printf(GenerateResponse, input, authorUsername)
	if client.Client == nil {
		return "", fmt.Errorf(ErrUninitOpenAI.Error())
	}

	if authorUsername == "" {
		return "", errors.New("author username cannot be empty")
	}

	if strings.Contains(authorUsername, " ") {
		return "", errors.New("author username cannot contain spaces")
	}

	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf(ErrEmptyInput.Error())
	}

	cacheKey := authorUsername
	cacheValue, ok := client.CacheContains(cacheKey)
	userCacheItem, _ := cacheValue.(UserCacheItem)

	systemMessage := fmt.Sprintf("[PROMPT]%s[/PROMPT] Conversation with: %s[CONVERSATION]", client.Config.DefaultPromptFile, authorUsername)
	var conversation []openai.ChatCompletionMessage
	isNewConversation := false

	if !ok || len(userCacheItem.Conversations) == 0 || userCacheItem.Conversations[len(userCacheItem.Conversations)-1].IsFinished {
		conversation = append(conversation, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: systemMessage,
		})
		isNewConversation = true
	} else {
		conversation = userCacheItem.Conversations[len(userCacheItem.Conversations)-1].Conversation
	}

	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: input,
	})

	ctx := context.Background()

	req := client.createChatCompletionRequest(conversation)

	response, isFinished, err := client.performChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	conversation = append(conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: response,
	})

	if isNewConversation {
		userCacheItem.Conversations = append(userCacheItem.Conversations, CacheItem{
			Conversation: conversation,
			Timestamp:    time.Now(),
			IsFinished:   isFinished,
		})
		if len(userCacheItem.Conversations) > ConversationCacheSize {
			userCacheItem.Conversations = userCacheItem.Conversations[1:]
		}
	} else {
		userCacheItem.Conversations[len(userCacheItem.Conversations)-1].Conversation = conversation
		userCacheItem.Conversations[len(userCacheItem.Conversations)-1].IsFinished = isFinished
	}

	client.AddItemToCache(cacheKey, userCacheItem)

	return response, nil
}

func (client *OpenAiContext) createChatCompletionRequest(conversation []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo,
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

func (client *OpenAiContext) performChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (string, bool, error) {
	var allResponses strings.Builder

	for {
		respInterface, err := retryWithBackoff(func() (interface{}, error) {
			client.sem <- struct{}{}        // Acquire semaphore
			defer func() { <-client.sem }() // Ensure semaphore release
			return client.Client.CreateChatCompletion(ctx, req)
		}, maxRetries)
		if err != nil {
			return "", false, fmt.Errorf("%s %v", ErrFailedChatComplete, err)
		}

		response, ok := respInterface.(openai.ChatCompletionResponse)
		if !ok {
			return "", false, errors.New("failed to cast to openai.ChatCompletionResponse")
		}

		if len(response.Choices) == 0 {
			return "", false, fmt.Errorf(ErrNoChoicesResponse.Error())
		}

		responseText := response.Choices[0].Message.Content
		finishReason := response.Choices[0].FinishReason

		allResponses.WriteString(responseText)

		if finishReason == openai.FinishReasonStop {
			return allResponses.String(), false, nil
		} else {
			return allResponses.String(), true, nil
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
		return false, fmt.Errorf(ErrUninitOpenAI.Error())
	}

	if strings.TrimSpace(input) == "" {
		return false, fmt.Errorf(ErrEmptyInput.Error())
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
			return false, fmt.Errorf(ErrMaxRetries.Error())
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
			return false, fmt.Errorf(ErrNoModResults.Error())
		}

		return resp.Results[0].Flagged, nil
	}
}

func (client *OpenAiContext) Close() {
	close(client.sem)
}

func (client *OpenAiContext) RunCacheEviction() {
	ticker := time.NewTicker(client.Config.CacheLifeTime)

	for {
		<-ticker.C
		cachedItemsCopy := make(map[interface{}]UserCacheItem)

		client.generationCache.Range(func(k, v interface{}) bool {
			userCacheItem, _ := v.(UserCacheItem)
			cachedItemsCopy[k] = userCacheItem
			return true
		})

		for key, userCacheItem := range cachedItemsCopy {
			for i, v := range userCacheItem.Conversations {
				if time.Since(v.Timestamp) > client.Config.CacheLifeTime {
					if len(userCacheItem.Conversations) == 1 {
						client.DeleteItemFromCache(key)
					} else {
						userCacheItem.Conversations = append(userCacheItem.Conversations[:i], userCacheItem.Conversations[i+1:]...)
						client.AddItemToCache(key, userCacheItem)
					}
					break
				}
			}
		}
	}
}
