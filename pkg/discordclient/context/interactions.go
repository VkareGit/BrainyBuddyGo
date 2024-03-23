package discordclient

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

func (dc *DiscordContext) sendTestComponents(channelID string, reference *discordgo.MessageReference, username string, tag string) {
	start, end := 0, 5
	matchDetails, err := dc.handleHistoryRequest(username, tag, start, end)
	if err != nil {
		log.Printf("Failed to get history: %v", err)
		return
	}

	description := dc.formatMatchDetails(matchDetails)

	nextButton := dc.createButton("show_next_button", "Show Next", "➡️", username, tag, start, end)
	previousButton := dc.createButton("show_previous_button", "Show Previous", "⬅️", username, tag, start, end)

	components := []discordgo.MessageComponent{
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&previousButton, &nextButton}},
	}

	embed := discordgo.MessageEmbed{
		Title:       fmt.Sprintf("History (Page %d)", start/5+1),
		Description: description,
	}

	msg := discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{&embed},
		Components: components,
		Reference:  reference,
	}

	_, err = dc.Session.ChannelMessageSendComplex(channelID, &msg)
	if err != nil {
		log.Printf("Failed to send test components: %v", err)
	}
}

func (dc *DiscordContext) interactionCreateHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionMessageComponent {
		logrus.Info("Received a message component interaction")

		customID := i.MessageComponentData().CustomID
		action, username, tag, start, end, err := dc.parseCustomID(customID)
		if err != nil {
			logrus.Error("Invalid customID")
			return
		}

		if action == "show_next_button" || action == "show_previous_button" {
			logrus.Infof("The custom ID of the interaction is '%s'", customID)

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
			if err != nil {
				logrus.WithError(err).Error("Failed to acknowledge interaction")
				return
			}

			if action == "show_next_button" {
				start = end
				end = start + 5
			} else if action == "show_previous_button" {
				end = start
				start = end - 5
				if start < 0 {
					start = 0
				}
			}

			matchDetails, err := dc.handleHistoryRequest(username, tag, start, end)
			if err != nil {
				logrus.WithError(err).Error("Failed to get history")
			}
			title := fmt.Sprintf("History (Page %d)", start/5+1)
			description := dc.formatMatchDetails(matchDetails)

			embed := discordgo.MessageEmbed{
				Title:       title,
				Description: description,
			}

			nextButton := dc.createButton("show_next_button", "Show Next", "➡️", username, tag, start, end)
			previousButton := dc.createButton("show_previous_button", "Show Previous", "⬅️", username, tag, start, end)

			components := []discordgo.MessageComponent{
				&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&previousButton, &nextButton}},
			}

			edit := &discordgo.MessageEdit{
				ID:         i.Message.ID,
				Channel:    i.ChannelID,
				Embed:      &embed,
				Components: components,
			}

			_, err = s.ChannelMessageEditComplex(edit)
			if err != nil {
				logrus.WithError(err).Error("Failed to edit message")
			} else {
				logrus.Info("Edited the message successfully")
			}
		} else {
			logrus.Infof("The custom ID of the interaction is not 'show_next_button' or 'show_previous_button', it's '%s'", customID)
		}
	} else {
		logrus.Infof("Received an interaction of type %d, not a message component interaction", i.Type)
	}
}

func (dc *DiscordContext) formatMatchDetails(matchDetails []MatchDetail) string {
	description := "Here is the history:\n"
	for _, match := range matchDetails {
		matchDescription := fmt.Sprintf("Match ID: %s, Game Mode: %s, Game Duration: %d seconds\n", match.MatchID, match.GameMode, match.GameDuration)
		for _, participant := range match.Participants {
			participantDescription := fmt.Sprintf("Participant ID: %d, Champion: %d\n", participant.ParticipantID, participant.ChampionID)
			if len(description)+len(matchDescription)+len(participantDescription) > 2048 {
				break
			}
			matchDescription += participantDescription
		}
		if len(description)+len(matchDescription) > 2048 {
			break
		}
		description += matchDescription
	}
	return description
}

func (dc *DiscordContext) createButton(action, label, emoji, username, tag string, start, end int) discordgo.Button {
	return discordgo.Button{
		Label:    label,
		Style:    discordgo.PrimaryButton,
		CustomID: fmt.Sprintf("%s:%s:%s:%d:%d", action, username, tag, start, end),
		Emoji: discordgo.ComponentEmoji{
			Name: emoji,
		},
	}
}

func (dc *DiscordContext) parseCustomID(customID string) (action, username, tag string, start, end int, err error) {
	split := strings.Split(customID, ":")
	if len(split) < 5 {
		return "", "", "", 0, 0, fmt.Errorf("invalid customID")
	}
	action, username, tag = split[0], split[1], split[2]
	start, err = strconv.Atoi(split[3])
	if err != nil {
		return "", "", "", 0, 0, err
	}
	end, err = strconv.Atoi(split[4])
	if err != nil {
		return "", "", "", 0, 0, err
	}
	return action, username, tag, start, end, nil
}
