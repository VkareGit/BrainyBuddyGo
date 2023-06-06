package context

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
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

type IsQuestionJob struct {
	input  string
	result chan bool
}

type OpenAiContext struct {
	APIKey             string
	Prompt             string
	Client             *openai.Client
	workers            int
	sem                chan struct{}
	isQuestionJobQueue chan IsQuestionJob
	stop               chan bool
	mutex              sync.Mutex
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
		APIKey:             apiKey,
		Prompt:             string(prompt),
		Client:             openai.NewClient(apiKey),
		workers:            workers,
		sem:                make(chan struct{}, workers),
		isQuestionJobQueue: make(chan IsQuestionJob, workers),
		stop:               make(chan bool, 1),
	}

	go client.processIsQuestionJobs()

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

func (client *OpenAiContext) processIsQuestionJobs() {
	for {
		select {
		case job := <-client.isQuestionJobQueue:
			client.mutex.Lock()
			isQuestion, err := client.callIsQuestionAPI(job.input)
			client.mutex.Unlock()
			if err != nil {
				log.Printf("Failed to check if %s is a question: %v", job.input, err)
				job.result <- false
				continue
			}
			job.result <- isQuestion
		case <-client.stop:
			return
		}
	}
}

func (client *OpenAiContext) callIsQuestionAPI(input string) (bool, error) {
	jsonData := map[string]string{
		"sentence": input,
	}
	jsonValue, _ := json.Marshal(jsonData)

	response, err := http.Post("http://164.90.187.110:8080", "application/json", bytes.NewBuffer(jsonValue))

	if err != nil {
		log.Printf("The HTTP request failed with error %s\n", err)
		return false, err
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, err
	}

	var result map[string]bool
	err = json.Unmarshal([]byte(data), &result)
	if err != nil {
		return false, err
	}

	return result["is_question"], nil
}

func (client *OpenAiContext) IsQuestionAPI(input string) (bool, error) {
	result := make(chan bool)
	client.isQuestionJobQueue <- IsQuestionJob{input: input, result: result}
	return <-result, nil
}

func (client *OpenAiContext) Close() {
	close(client.stop)
	close(client.sem)
	close(client.isQuestionJobQueue)
}
