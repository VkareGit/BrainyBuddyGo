package context

import (
	"time"

	"github.com/sashabaranov/go-openai"
)

type CacheItem struct {
	Conversation []openai.ChatCompletionMessage
	Timestamp    time.Time
	IsFinished   bool
}

type UserCacheItem struct {
	Conversations []CacheItem
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
