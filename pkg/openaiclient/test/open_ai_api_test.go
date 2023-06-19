package context_test

import (
	contextpkg "BrainyBuddyGo/pkg/openaiclient/context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

func setBasepath() string {
	_, filename, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	basepath = strings.Join(strings.Split(basepath, "/")[:len(strings.Split(basepath, "/"))-1], "/")
	return basepath
}
func loadEnvFile(basepath string) {
	envFilePath := filepath.Join(basepath, ".env")
	err := godotenv.Load(envFilePath)
	if err != nil {
		log.Printf("Failed to load .env file: %s\n", err)
	}
}

func getOpenAiToken() (string, error) {
	openAiToken := os.Getenv("OPENAI_API_KEY")
	if openAiToken == "" {
		return "", errors.New("OPENAI_API_KEY not set")
	}
	return openAiToken, nil
}

func isProduciton() (bool, error) {
	production := false
	productionEnv := os.Getenv("PRODUCTION")
	if productionEnv == "true" {
		production = true
	}
	return production, nil
}

func getOpenAiContext() (*contextpkg.OpenAiContext, error) {
	basepath := setBasepath()
	loadEnvFile(basepath)
	openAiToken, err := getOpenAiToken()
	if err != nil {
		return nil, err
	}

	isProduciton, err := isProduciton()
	if err != nil {
		return nil, err
	}

	apiKey := openAiToken
	workers := 1
	ctx, err := contextpkg.NewOpenAiContext(apiKey, workers, basepath, isProduciton)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func TestNewOpenAiContext(t *testing.T) {
	_, err := getOpenAiContext()
	if err != nil {
		t.Fatal(err)
	}
}

func TestProcessMessageSucces(t *testing.T) {
	ctx, err := getOpenAiContext()
	if err != nil {
		t.Fatal(err)
	}

	message := "Hi how are you?"
	result := true
	result, err = ctx.ModerationCheck(message, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Fatalf("Expected result fail but was true")
	}
}

func TestProcessMessageFail(t *testing.T) {
	ctx, err := getOpenAiContext()
	if err != nil {
		t.Fatal(err)
	}

	message := "I want to kill this noobs"
	result := false
	result, err = ctx.ModerationCheck(message, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Fatalf("Expected result fail but was true")
	}
}

func TestGenerateResponse(t *testing.T) {
	ctx, err := getOpenAiContext()
	if err != nil {
		t.Fatal(err)
	}

	message := "Hi how are you?"
	result := ""
	result, err = ctx.GenerateResponse(message, "testUser")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatalf("Expected result not empty but was empty")
	}
}

func TestCacheContains(t *testing.T) {
	ctx, err := getOpenAiContext()
	if err != nil {
		t.Fatal(err)
	}

	cacheItem := contextpkg.CacheItem{
		Conversation: []openai.ChatCompletionMessage{},
		Timestamp:    time.Now(),
		IsFinished:   false,
	}

	userCacheItem := contextpkg.UserCacheItem{
		Conversations: []contextpkg.CacheItem{cacheItem},
	}

	ctx.AddItemToCache("testKey", userCacheItem)

	_, ok := ctx.CacheContains("testKey")
	if !ok {
		t.Fatalf("Cache item was not found")
	}
}

func TestRunCacheEviction(t *testing.T) {
	ctx, err := getOpenAiContext()
	if err != nil {
		t.Fatal(err)
	}

	ctx.Config.CacheLifeTime = 2 * time.Second

	cacheItem := contextpkg.CacheItem{
		Conversation: []openai.ChatCompletionMessage{},
		Timestamp:    time.Now(),
		IsFinished:   false,
	}

	userCacheItem := contextpkg.UserCacheItem{
		Conversations: []contextpkg.CacheItem{cacheItem},
	}

	ctx.AddItemToCache("testKey", userCacheItem)

	time.Sleep(3 * time.Second)

	_, ok := ctx.CacheContains("testKey")
	if ok {
		t.Fatalf("Cache item was not evicted")
	}
}
