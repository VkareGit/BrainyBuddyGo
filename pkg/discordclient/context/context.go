package discordclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	openai "BrainyBuddyGo/pkg/openaiclient/context"

	"github.com/bwmarrin/discordgo"
)

const (
	UnableToAssistMsg = "I'm sorry, but I'm not able to assist at this time."
	CantAnswerNowMsg  = "Sorry, I can't answer that question right now."
)

var AllowedChannels = []string{
	"1122558947941945354",
	"1114708430859550771",
	"1113239460281335820",
}

var AdminUsers = []string{
	"530482157571932184",
	"246295908545724417",
	"404772146330599424",
}

type MessageLimiter interface {
	RegisterMessage(userID string) (bool, time.Duration)
}

type DiscordContext struct {
	Session   *discordgo.Session
	AIContext *openai.OpenAiContext
	Limiter   MessageLimiter
}

func NewDiscordContext(ctx context.Context, discordToken string, aiContext *openai.OpenAiContext, limiter MessageLimiter) (*DiscordContext, error) {
	if discordToken == "" {
		return nil, errors.New("discord token is empty")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	dc := &DiscordContext{
		Session:   dg,
		AIContext: aiContext,
		Limiter:   limiter,
	}

	dc.Session.AddHandler(dc.ready)
	dc.Session.AddHandler(dc.messageCreateHandler)

	return dc, nil
}

func (dc *DiscordContext) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot is ready with the following guilds:")
	for _, guild := range event.Guilds {
		log.Printf(" - %s", guild.Name)
	}
}

func (dc *DiscordContext) messageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	go dc.handleNewMessage(s, m)
}

func isAllowedChannel(channelID string) bool {
	for _, id := range AllowedChannels {
		if channelID == id {
			return true
		}
	}
	return false
}

func isAdmin(userID string) bool {
	for _, id := range AdminUsers {
		if userID == id {
			return true
		}
	}
	return false
}

func (dc *DiscordContext) handleNewMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || isAdmin(m.Author.ID) || !isAllowedChannel(m.ChannelID) {
		return
	}

	log.Printf("Message from %s saying %s in channel %s", m.Author.Username, m.Content, m.ChannelID)

	response, err := dc.generateAIResponse(m.Content, m.Author.Username)
	if err != nil {
		log.Printf("Failed to generate response: %v", err)
		return
	}

	if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func (dc *DiscordContext) generateAIResponse(question string, authorUsername string) (string, error) {
	ok, timeLeft := dc.Limiter.RegisterMessage(authorUsername)
	if !ok {
		return fmt.Sprintf("Sorry, you can ask another question in %.0f minutes", timeLeft.Minutes()), nil
	}

	response, err := dc.AIContext.AnswerQuestion(context.Background(), question)
	if err != nil {
		log.Printf("Failed to generate response for question from %s: %v", authorUsername, err)
		return CantAnswerNowMsg, err
	}
	return response, nil
}

func (dc *DiscordContext) OpenConnection() error {
	if dc.Session == nil {
		return errors.New("session is not initialized")
	}

	if err := dc.Session.Open(); err != nil {
		log.Printf("Unable to open connection: %v", err)
		return err
	}

	return nil
}

func (dc *DiscordContext) CloseConnection() error {
	if dc.Session == nil {
		return errors.New("session is not initialized")
	}

	if err := dc.Session.Close(); err != nil {
		log.Printf("Unable to close connection: %v", err)
		return err
	}

	return nil
}
