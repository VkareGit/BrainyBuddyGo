package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	config "BrainyBuddyGo/Config"
	discordclient "BrainyBuddyGo/pkg/discordclient/context"
	limiter "BrainyBuddyGo/pkg/discordclient/limiter"
	openAiContext "BrainyBuddyGo/pkg/openaiclient/context"
	riotapi "BrainyBuddyGo/pkg/riotclient/context"
)

const (
	OpenAiThreadsNumber = 5
)

type Bot struct {
	discordCtx *discordclient.DiscordContext
	openAiCtx  *openAiContext.OpenAiContext
	Limiter    *limiter.MessageLimiter
	riotCtx    *riotapi.RiotContext
}

func NewBot(ctx context.Context, cfg *config.Configuration) (*Bot, error) {
	oa, err := openAiContext.NewClient(cfg.OpenAiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAi context: %w", err)
	}

	lim := limiter.NewMessageLimiter()

	riotCtx := riotapi.NewRiotAPI(cfg.RiotApiKey)

	b := &Bot{
		openAiCtx: oa,
		Limiter:   lim,
		riotCtx:   riotCtx,
	}

	dc, err := discordclient.NewDiscordContext(ctx, cfg.DiscordToken, oa, riotCtx, lim)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Discord context: %w", err)
	}

	if err := dc.OpenConnection(); err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	b.discordCtx = dc
	return b, nil
}

func (b *Bot) Close(ctx context.Context) error {
	err := b.discordCtx.CloseConnection()
	if err != nil {
		return fmt.Errorf("failed to close Discord context connection: %w", err)
	}

	log.Println("Discord context and OpenAI context closed successfully")

	return nil
}

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	b, err := NewBot(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	defer func() {
		if err := b.Close(ctx); err != nil {
			log.Fatalf("Failed to close connection: %v", err)
		}
	}()

	// Wait for a termination signal while the bot is running
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
