package context

import "errors"

var (
	ErrEmptyAPIKey        = errors.New("OpenAI API Key is empty")
	ErrUninitOpenAI       = errors.New("OpenAI client is not initialized")
	ErrFailedChatComplete = errors.New("failed to create chat completion")
	ErrNoChoicesResponse  = errors.New("no choices in response")
	ErrNonEnglishInput    = errors.New("input is not in English")
	ErrFailedModeration   = errors.New("failed to moderate text")
	ErrNoModResults       = errors.New("no choices were returned in the moderation response")
	ErrMaxRetries         = errors.New("failed to moderate text after maximum retries")
	ErrEmptyInput         = errors.New("input is empty")
)
