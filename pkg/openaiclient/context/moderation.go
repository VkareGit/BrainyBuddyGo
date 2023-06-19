package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/sashabaranov/go-openai"
)

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
