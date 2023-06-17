package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	config "BrainyBuddyGo/Config"
	discordContext "BrainyBuddyGo/pkg/discordclient/context"
	openAiContext "BrainyBuddyGo/pkg/openaiclient/context"
)

const (
	OpenAiThreadsNumber = 5
)

type Bot struct {
	discordCtx *discordContext.DiscordContext
	openAiCtx  *openAiContext.OpenAiContext
}

func NewBot(cfg *config.Configuration) (*Bot, error) {
	oa, err := openAiContext.NewOpenAiContext(cfg.OpenAiToken, OpenAiThreadsNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAi context: %w", err)
	}

	dc, err := discordContext.Initialize(cfg.DiscordToken, oa)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Discord context: %w", err)
	}

	if err := dc.OpenConnection(); err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	return &Bot{
		discordCtx: dc,
		openAiCtx:  oa,
	}, nil
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	b, err := NewBot(cfg)
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
