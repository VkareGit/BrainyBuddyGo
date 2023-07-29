package limiter

import (
	"container/list"
	"sync"
	"time"
)

const MaxMessages = 5
const LimitDuration = 3 * time.Hour

type MessageLimiter struct {
	userMessages map[string]*list.List
	mutex        sync.RWMutex
}

func NewMessageLimiter() *MessageLimiter {
	return &MessageLimiter{
		userMessages: make(map[string]*list.List),
	}
}

func (m *MessageLimiter) RegisterMessage(userID string) (bool, time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()

	// Use a linked list as a queue to store timestamps
	messages, exists := m.userMessages[userID]
	if !exists {
		messages = list.New()
		m.userMessages[userID] = messages
	}

	// Remove old timestamps from the front of the queue
	for messages.Len() > 0 {
		first := messages.Front()
		if now.Sub(first.Value.(time.Time)) > LimitDuration {
			messages.Remove(first)
		} else {
			break
		}
	}

	// Check if user can send more messages
	if messages.Len() < MaxMessages {
		messages.PushBack(now)
		return true, 0
	} else {
		// If not, calculate how much longer they need to wait
		first := messages.Front()
		return false, LimitDuration - now.Sub(first.Value.(time.Time))
	}
}
