package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/cenkalti/backoff/v4"
	"github.com/chrisport/go-lang-detector/langdet"
	"github.com/chrisport/go-lang-detector/langdet/langdetdef"
)

func getWorkerCount(workers int) int {
	if workers <= 0 {
		return 1
	}
	return workers
}

func getPrompt(promptData []byte, production bool) (string, error) {
	var allPrompts []string

	if !production {
		var config NormalPromptData
		err := json.Unmarshal(promptData, &config)
		if err != nil {
			return "", fmt.Errorf("failed to decode config file: %w", err)
		}
		allPrompts = append(allPrompts, strings.Join(config.Welcome, " "))
	} else {
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
	}

	return strings.Join(allPrompts, " "), nil
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

func checkLanguage(input string) error {
	detector := langdet.NewDetector()
	detector.AddLanguageComparators(langdetdef.ENGLISH)

	detectedLanguage := detector.GetClosestLanguage(input)
	if detectedLanguage != "english" {
		return fmt.Errorf("%s, detected language is: %s", ErrNonEnglishInput, detectedLanguage)
	}

	return nil
}
