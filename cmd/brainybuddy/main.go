package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	config "BrainyBuddyGo/Config"
	DiscordContext_ "BrainyBuddyGo/pkg/discordclient/context"
	OpenAiConext_ "BrainyBuddyGo/pkg/openaiclient/context"
)

const (
	OpenAiThreadsNumber = 5
)

type Bot struct {
	discordContext *DiscordContext_.DiscordContext
	openAiContext  *OpenAiConext_.OpenAiContext
}

func NewBot(cfg *config.Configuration) (*Bot, error) {
	oa, err := OpenAiConext_.Initialize(cfg.OpenAiToken, OpenAiThreadsNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAi context: %w", err)
	}

	dc, err := DiscordContext_.Initialize(cfg.DiscordToken, oa)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Discord context: %w", err)
	}

	if err := dc.OpenConnection(); err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	return &Bot{
		discordContext: dc,
		openAiContext:  oa,
	}, nil
}

func (b *Bot) Close() error {
	return b.discordContext.CloseConnection()
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

	// Wait for a termination signal while the bot is running
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	if err := b.Close(); err != nil {
		log.Fatalf("Failed to close connection: %v", err)
	}
}
