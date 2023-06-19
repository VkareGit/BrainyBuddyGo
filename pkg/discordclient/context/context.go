package context

import (
	"errors"
	"log"

	"BrainyBuddyGo/pkg/discordclient/handler"
	aiContext "BrainyBuddyGo/pkg/openaiclient/context"

	"github.com/bwmarrin/discordgo"
)

type DiscordContext struct {
	Session   *discordgo.Session
	Handler   *handler.Handler
	AIContext *aiContext.OpenAiContext
}

func (dc *DiscordContext) RegisterHandlers() {
	dc.Session.AddHandler(handler.Ready)
	dc.Session.AddHandler(dc.Handler.MessageCreateHandler)
}

func Initialize(discordToken string, aiContext *aiContext.OpenAiContext, limiter handler.MessageLimiter) (*DiscordContext, error) {
	if discordToken == "" {
		return nil, errors.New("discord token is empty")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		err = errors.New("Error creating Discord session: " + err.Error())
		log.Println(err)
		return nil, err
	}

	handler := handler.NewHandler(aiContext, limiter)

	dc := &DiscordContext{
		Session:   dg,
		Handler:   handler,
		AIContext: aiContext,
	}

	dc.RegisterHandlers()

	return dc, nil
}

func (dc *DiscordContext) checkSessionInitialized() error {
	if dc.Session == nil {
		return errors.New("session is not initialized")
	}
	return nil
}

func (dc *DiscordContext) OpenConnection() error {
	if err := dc.checkSessionInitialized(); err != nil {
		return err
	}

	if err := dc.Session.Open(); err != nil {
		log.Printf("Unable to open connection: %v", err)
		return err
	}

	return nil
}

func (dc *DiscordContext) CloseConnection() error {
	if err := dc.checkSessionInitialized(); err != nil {
		return err
	}

	if err := dc.Session.Close(); err != nil {
		log.Printf("Unable to close connection: %v", err)
		return err
	}

	return nil
}
