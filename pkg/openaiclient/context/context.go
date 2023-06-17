package context

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
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

type OpenAiContext struct {
	APIKey  string
	Prompt  string
	Client  *openai.Client
	workers int
	sem     chan struct{}
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
		APIKey:  apiKey,
		Prompt:  prompt,
		Client:  openai.NewClient(apiKey),
		workers: getWorkerCount(workers),
		sem:     make(chan struct{}, workers),
	}

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

	ctx := context.Background()

	systemMessage := fmt.Sprintf(client.Prompt, authorUsername)

	req := client.createChatCompletionRequest(systemMessage, input)

	return client.performChatCompletion(ctx, req)
}

func (client *OpenAiContext) createChatCompletionRequest(systemMessage string, input string) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo16K,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: systemMessage,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: input,
			},
		},
		MaxTokens:   DefaultMaxTokens,
		N:           DefaultN,
		Temperature: DefaultTemperature,
	}
}

func (client *OpenAiContext) performChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	bo := backoff.NewExponentialBackOff()
	for {
		client.sem <- struct{}{}
		resp, err := client.Client.CreateChatCompletion(ctx, req)
		<-client.sem

		if err != nil {
			nextInterval := bo.NextBackOff()
			if nextInterval != backoff.Stop {
				time.Sleep(nextInterval)
				continue
			}
			return "", fmt.Errorf("%s %v", ErrFailedChatComplete, err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf(ErrNoChoicesResponse)
		}

		return resp.Choices[0].Message.Content, nil
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

	return client.performModeration(ctx, req, maxRetries)
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
