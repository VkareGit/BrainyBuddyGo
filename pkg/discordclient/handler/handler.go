package handler

import (
	"log"

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

type Handler struct {
	AIContext *aiContext.OpenAiContext
}

func NewHandler(aiContext *aiContext.OpenAiContext) *Handler {
	return &Handler{
		AIContext: aiContext,
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
		log.Println("AIContext is not initialized")
		return
	}

	response := h.GenerateAIResponse(m.Content, m.Author.Username)
	if response != "" {
		if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
			log.Printf("Failed to send message: %v", err)
		}
	}
}

func (h *Handler) GenerateAIResponse(question string, authorUsername string) string {
	if h.AIContext == nil {
		log.Println("AIContext is not initialized")
		return UnableToAssistMsg
	}

	ModerateQuestion, err := h.AIContext.ModerationCheck(question, authorUsername, ModerateQuestionMaxRetries)

	if err != nil {
		log.Printf("Failed to moderate question from %s : %v", authorUsername, err)
		return CantAnswerNowMsg
	}

	if ModerateQuestion {
		return UnableToAssistMsg
	}

	response, err := h.AIContext.GenerateResponse(question, authorUsername)
	if err != nil {
		log.Printf("Failed to generate response for question from %s: %v", authorUsername, err)
		return CantAnswerNowMsg
	}
	return response
}
