package context

import (
	"errors"
	"log"

	isQuestionContext "BrainyBuddyGo/pkg/apiclient/context"
	handler "BrainyBuddyGo/pkg/discordclient/handler"
	aiContext "BrainyBuddyGo/pkg/openaiclient/context"

	"github.com/bwmarrin/discordgo"
)

type DiscordContext struct {
	Session           *discordgo.Session
	Handler           *handler.Handler
	AIContext         *aiContext.OpenAiContext
	ISQuestionContext *isQuestionContext.IsQuestionContext
}

func (dc *DiscordContext) RegisterHandlers() {
	dc.Session.AddHandler(handler.Ready)
	dc.Session.AddHandler(dc.Handler.MessageCreateHandler)
}

func Initialize(discordToken string, aiContext *aiContext.OpenAiContext, isQuestionContext *isQuestionContext.IsQuestionContext) (*DiscordContext, error) {
	if discordToken == "" {
		return nil, errors.New("Discord token is empty")
	}

	handler := handler.NewHandler(aiContext, isQuestionContext)

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Printf("Error creating Discord session: %v", err)
		return nil, err
	}

	dc := &DiscordContext{
		Session:           dg,
		Handler:           handler,
		AIContext:         aiContext,
		ISQuestionContext: isQuestionContext,
	}

	dc.RegisterHandlers()

	return dc, nil
}

func (dc *DiscordContext) OpenConnection() error {
	if dc.Session == nil {
		return errors.New("Session is not initialized")
	}

	if err := dc.Session.Open(); err != nil {
		log.Printf("Unable to open connection: %v", err)
		return err
	}

	return nil
}

func (dc *DiscordContext) CloseConnection() error {
	if dc.Session == nil {
		return errors.New("Session is not initialized")
	}

	if err := dc.Session.Close(); err != nil {
		log.Printf("Unable to close connection: %v", err)
		return err
	}

	return nil
}
