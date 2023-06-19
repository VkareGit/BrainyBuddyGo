package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	config "BrainyBuddyGo/Config"
	discordContext "BrainyBuddyGo/pkg/discordclient/context"
	"BrainyBuddyGo/pkg/discordclient/limiter"
	openAiContext "BrainyBuddyGo/pkg/openaiclient/context"
)

const (
	OpenAiThreadsNumber = 5
)

type Bot struct {
	discordCtx *discordContext.DiscordContext
	openAiCtx  *openAiContext.OpenAiContext
	Limiter    *limiter.MessageLimiter
}

func NewBot(cfg *config.Configuration, basepath string) (*Bot, error) {
	oa, err := openAiContext.NewOpenAiContext(cfg.OpenAiToken, OpenAiThreadsNumber, basepath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAi context: %w", err)
	}

	lim := limiter.NewMessageLimiter()

	b := &Bot{
		openAiCtx: oa,
		Limiter:   lim,
	}

	dc, err := discordContext.Initialize(cfg.DiscordToken, oa, lim)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Discord context: %w", err)
	}

	if err := dc.OpenConnection(); err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	b.discordCtx = dc
	return b, nil
}

func (b *Bot) Close() error {
	err := b.discordCtx.CloseConnection()
	if err != nil {
		return fmt.Errorf("failed to close Discord context connection: %w", err)
	}

	b.openAiCtx.Close()
	log.Println("OpenAI context closed successfully")

	return nil
}

func main() {
	_, filename, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(filepath.Dir(filepath.Dir(filename)))

	cfg, err := config.Load(basepath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	b, err := NewBot(cfg, basepath)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	defer func() {
		if err := b.Close(); err != nil {
			log.Fatalf("Failed to close connection: %v", err)
		}
	}()

	// Wait for a termination signal while the bot is running
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
