package context

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

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
