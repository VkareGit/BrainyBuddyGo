package handler

import (
	"fmt"
	"log"
	"time"

	aiContext "BrainyBuddyGo/pkg/openaiclient/context"

	"github.com/bwmarrin/discordgo"
)

const (
	ModerateQuestionMaxRetries = 3
	UnableToAssistMsg          = "I'm sorry, but I'm not able to assist at this time."
	CantAnswerNowMsg           = "Sorry, I can't answer that question right now."
)

var allowedChannels = []string{
	"1114708430859550771",
}

type MessageLimiter interface {
	RegisterMessage(userID string) (bool, time.Duration)
}

type Handler struct {
	AIContext *aiContext.OpenAiContext
	Limiter   MessageLimiter
}

func NewHandler(aiContext *aiContext.OpenAiContext, limiter MessageLimiter) *Handler {
	return &Handler{
		AIContext: aiContext,
		Limiter:   limiter,
	}
}

func Ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot is ready with the following guilds:")
	for _, guild := range event.Guilds {
		log.Printf(" - %s", guild.Name)
	}
}

func isAllowedChannel(channelID string) bool {
	for _, id := range allowedChannels {
		if channelID == id {
			return true
		}
	}
	return false
}

func (h *Handler) MessageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if !isAllowedChannel(m.ChannelID) {
		return
	}

	log.Printf("Message from %s saying %s in channel %s", m.Author.Username, m.Content, m.ChannelID)

	if h.AIContext == nil {
		log.Println(aiContext.ErrUninitOpenAI)
		return
	}

	response, err := h.GenerateAIResponse(m.Content, m.Author.Username)
	if err != nil {
		log.Printf("Failed to generate response: %v", err)
		return
	}

	if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func (h *Handler) GenerateAIResponse(question string, authorUsername string) (string, error) {
	if h.AIContext == nil {
		return UnableToAssistMsg, fmt.Errorf(aiContext.ErrUninitOpenAI.Error())
	}

	ok, timeLeft := h.Limiter.RegisterMessage(authorUsername)
	if !ok {
		return fmt.Sprintf("Sorry, you can ask another question in %.0f minutes", timeLeft.Minutes()), nil
	}

	flagged, err := h.AIContext.ModerationCheck(question, ModerateQuestionMaxRetries)
	if err != nil {
		log.Printf("Failed to moderate question from %s : %v", authorUsername, err)
		return CantAnswerNowMsg, err
	}

	if flagged {
		return UnableToAssistMsg, nil
	}

	response, err := h.AIContext.GenerateResponse(question, authorUsername)
	if err != nil {
		log.Printf("Failed to generate response for question from %s: %v", authorUsername, err)
		return CantAnswerNowMsg, err
	}
	return response, nil
}
