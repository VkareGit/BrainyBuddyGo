package main

import (
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
		log.Printf("Failed to initialize OpenAi context: %v", err)
		return nil, err
	}

	dc, err := DiscordContext_.Initialize(cfg.DiscordToken, oa)
	if err != nil {
		log.Printf("Failed to initialize Discord context: %v", err)
		return nil, err
	}

	if err := dc.OpenConnection(); err != nil {
		log.Printf("Failed to open connection: %v", err)
		return nil, err
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
