package handler

import (
	"log"

	isQuestionContext "BrainyBuddyGo/pkg/apiclient/context"
	aiContext "BrainyBuddyGo/pkg/openaiclient/context"

	"github.com/bwmarrin/discordgo"
)

type Handler struct {
	AIContext         *aiContext.OpenAiContext
	ISQuestionContext *isQuestionContext.IsQuestionContext
}

func NewHandler(aiContext *aiContext.OpenAiContext, isQuestionContext *isQuestionContext.IsQuestionContext) *Handler {
	return &Handler{
		AIContext:         aiContext,
		ISQuestionContext: isQuestionContext,
	}
}

func Ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot is ready with the following guilds:")
	for _, guild := range event.Guilds {
		log.Printf(" - %s", guild.Name)
	}
}

func (h *Handler) MessageCreateHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("Message from %s saying %s in channel %s", m.Author.Username, m.Content, m.ChannelID)

	if h.AIContext == nil {
		log.Println("AIContext is not initialized")
		return
	}
	isQuestion, err := h.ISQuestionContext.IsQuestion(m.Content)
	if err != nil {
		log.Printf("Filed to check if input is a question: %v", err)
	}
	if isQuestion {
		log.Printf("Question: %s accepted", m.Content)
		response := h.GenerateAIResponse(m.Content, m.Author.Username)
		if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
			log.Printf("Failed to send message: %v", err)
		}
	}
}

func (h *Handler) GenerateAIResponse(question string, authorUsername string) string {
	if h.AIContext == nil {
		log.Println("AIContext is not initialized")
		return "I'm sorry, but I'm not able to assist at this time."
	}
	response, err := h.AIContext.GenerateResponse(question, authorUsername)
	if err != nil {
		log.Printf("Failed to generate response: %v", err)
		return "Sorry, I can't answer that question right now."
	}
	return response
}
