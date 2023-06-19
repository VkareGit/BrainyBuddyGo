package limiter

import (
	"sync"
	"time"
)

const MaxMessages = 5
const LimitDuration = 3 * time.Hour

type MessageLimiter struct {
	userMessages map[string][]time.Time
	mutex        sync.Mutex
}

func NewMessageLimiter() *MessageLimiter {
	return &MessageLimiter{
		userMessages: make(map[string][]time.Time),
	}
}

func (m *MessageLimiter) RegisterMessage(userID string) (bool, time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	messages, ok := m.userMessages[userID]
	now := time.Now()

	if ok {
		startIndex := 0
		for i, t := range messages {
			if now.Sub(t).Hours() <= LimitDuration.Hours() {
				startIndex = i
				break
			}
		}

		messages = messages[startIndex:]
	}

	messages = append(messages, now)
	m.userMessages[userID] = messages

	if len(messages) > MaxMessages {
		timeLeft := LimitDuration - now.Sub(messages[0])
		return false, timeLeft
	}

	return true, 0
}
