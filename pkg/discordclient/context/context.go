package discordclient

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	openai "BrainyBuddyGo/pkg/openaiclient/context"
	riotapi "BrainyBuddyGo/pkg/riotclient/context"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

const (
	UnableToAssistMsg = "I'm sorry, but I'm not able to assist at this time."
	CantAnswerNowMsg  = "Sorry, I can't answer that question right now."
)

var AllowedChannels = []string{
	"1122558947941945354",
	"1114708430859550771",
	"1113239460281335820",
	"1216197592975802388",
}

var AdminUsers = []string{}

type MessageLimiter interface {
	RegisterMessage(userID string) (bool, time.Duration)
}

type DiscordContext struct {
	Session     *discordgo.Session
	AIContext   *openai.OpenAiContext
	RiotContext *riotapi.RiotContext
	Limiter     MessageLimiter
}

func NewDiscordContext(ctx context.Context, discordToken string, aiContext *openai.OpenAiContext, riotContext *riotapi.RiotContext, limiter MessageLimiter) (*DiscordContext, error) {
	if discordToken == "" {
		return nil, errors.New("discord token is empty")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	dc := &DiscordContext{
		Session:     dg,
		AIContext:   aiContext,
		RiotContext: riotContext,
		Limiter:     limiter,
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

func (dc *DiscordContext) handleStatsRequest(gameName string, tagLine string) string {
	logrus.Infof("Fetching ranked stats for: %s#%s", gameName, tagLine)
	account, err := riotapi.GetAccountByRiotID(dc.RiotContext.APIKey, "Europe", gameName, tagLine)
	if err != nil {
		logrus.WithError(err).Error("Error retrieving account by Riot ID")
		return fmt.Sprintf("Error retrieving account: %s", err.Error())
	}
	logrus.Infof("Successfully fetched PUUID for %s#%s: %s", gameName, tagLine, account.Puuid)

	response, err := dc.RiotContext.GetPlayerRankedStats(account.Puuid)
	if err != nil {
		logrus.WithError(err).Error("Error retrieving ranked stats")
		return fmt.Sprintf("Error retrieving ranked stats: %s", err.Error())
	}

	logrus.Infof("Responding to ranked stats request for %s#%s", gameName, tagLine)
	return response
}

func (dc *DiscordContext) dispatchResponseActions(response string) string {
	if strings.Contains(response, "!STATS REQUEST!") {
		gameName, tagLine := dc.extractSummonerInfo(response)
		return dc.handleStatsRequest(gameName, tagLine)
	}

	return response
}

func (dc *DiscordContext) cleanString(s string) string {
	return strings.TrimFunc(strings.TrimSpace(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func (dc *DiscordContext) extractSummonerInfo(response string) (gameName, tagLine string) {
	startMarker := "!STATS REQUEST! "
	endMarker := " "
	startIndex := strings.Index(response, startMarker)

	if startIndex == -1 {
		return "", ""
	}

	summonerInfoSegment := response[startIndex+len(startMarker):]
	endIndex := strings.Index(summonerInfoSegment, endMarker)
	if endIndex != -1 {
		summonerInfoSegment = summonerInfoSegment[:endIndex]
	}

	if strings.Contains(summonerInfoSegment, "#") {
		parts := strings.SplitN(summonerInfoSegment, "#", 2)
		if len(parts) == 2 {
			gameName, tagLine = dc.cleanString(parts[0]), dc.cleanString(parts[1])
			return gameName, tagLine
		}
	}
	return "", ""
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

	response = dc.dispatchResponseActions(response)

	if _, err := s.ChannelMessageSendReply(m.ChannelID, response, m.Reference()); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func (dc *DiscordContext) generateAIResponse(question string, authorUsername string) (string, error) {
	ok, timeLeft := dc.Limiter.RegisterMessage(authorUsername)
	if !ok {
		return fmt.Sprintf("Sorry, you can ask another question in %.0f minutes", timeLeft.Minutes()), nil
	}

	response, err := dc.AIContext.GenerateAnswer(context.Background(), question)
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
