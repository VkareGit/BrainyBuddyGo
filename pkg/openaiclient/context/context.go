package context

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	openai "github.com/sashabaranov/go-openai"
)

const (
	DefaultMaxTokens   = 200
	DefaultN           = 1
	DefaultTemperature = 0.8
	DefaultPromptFile  = "pkg/openaiclient/context/config/prompt.txt"
)

type OpenAiContext struct {
	APIKey  string
	Prompt  string
	Client  *openai.Client
	workers int
	sem     chan struct{}
}

func Initialize(apiKey string, workers int) (*OpenAiContext, error) {
	if apiKey == "" {
		return nil, errors.New("OpenAi API Key is empty")
	}

	if workers <= 0 {
		workers = 1
	}

	promptPath, err := filepath.Abs(DefaultPromptFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for prompt file: %w", err)
	}

	prompt, err := ioutil.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file: %w", err)
	}

	client := &OpenAiContext{
		APIKey:  apiKey,
		Prompt:  string(prompt),
		Client:  openai.NewClient(apiKey),
		workers: workers,
		sem:     make(chan struct{}, workers),
	}

	return client, nil
}

func (client *OpenAiContext) GenerateResponse(input string, authorUsername string) (string, error) {
	log.Printf("Generating response for question: %s from user %s", input, authorUsername)
	if client.Client == nil {
		log.Println("OpenAI client is not initialized")
		return "", errors.New("OpenAI client is not initialized")
	}

	if strings.TrimSpace(input) == "" {
		return "", errors.New("input is empty")
	}

	ctx := context.Background()

	systemMessage := fmt.Sprintf(client.Prompt, authorUsername)

	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
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
			log.Println("Failed to create chat completion: ", err)
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no choices in response")
		}

		return resp.Choices[0].Message.Content, nil
	}
}

func (client *OpenAiContext) Close() {
	close(client.sem)
}
