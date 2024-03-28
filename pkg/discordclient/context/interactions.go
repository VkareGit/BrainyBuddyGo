package discordclient

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

func (dc *DiscordContext) formatBasicMatchDetails(matchDetails []MatchDetail, username string, tag string) string {
	var descriptionBuilder strings.Builder

	for i, match := range matchDetails {
		// Get the participant details for the user
		var participant ParticipantDetail
		for _, p := range append(match.BlueTeam.Participants, match.RedTeam.Participants...) {
			if p.SummonerName == username && p.SummonerTag == tag {
				participant = p
				break
			}
		}

		// Determine the team of the participant
		teamEmoji := "🔵"
		if match.Winner == "Red Team" {
			teamEmoji = "🔴"
		}

		championName := dc.resolveChampionID(participant.ChampionID)

		fmt.Fprintf(&descriptionBuilder, "%s %d - %s - %s %d/%d/%d - %s\n",
			teamEmoji, i+1, match.GameMode, championName, participant.Kills, participant.Deaths, participant.Assists, formatDuration(match.GameDuration))

		fmt.Fprintf(&descriptionBuilder, "KDA: %.2f, CS: %d (%.2f), DMG: %d, GOLD: %d\n\n",
			participant.KDA, participant.CS, float64(participant.CS)/float64(match.GameDuration/60), participant.DamageDealt, participant.Gold)
	}

	return descriptionBuilder.String()
}

func (dc *DiscordContext) sendTestComponents(channelID string, reference *discordgo.MessageReference, username string, tag string) {
	start, end := 0, 5
	matchDetails, err := dc.handleHistoryRequest(username, tag, start, end)
	if err != nil {
		log.Printf("Failed to get history: %v", err)
		return
	}

	description := dc.formatBasicMatchDetails(matchDetails, username, tag)

	nextButton := dc.createButton("show_next_button", "Next", "➡️", username, tag, start, end)
	previousButton := dc.createButton("show_previous_button", "Previous", "⬅️", username, tag, start, end)

	matchButtons := make([]discordgo.MessageComponent, len(matchDetails))
	for i := range matchDetails {
		matchButtons[i] = dc.createButton(fmt.Sprintf("show_match_%d", i), fmt.Sprintf("%d", i+1), "📜", username, tag, start, end)
	}

	components := []discordgo.MessageComponent{
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&previousButton, &nextButton}},
	}

	for i := 0; i < len(matchButtons); i += 5 {
		end := i + 5
		if end > len(matchButtons) {
			end = len(matchButtons)
		}
		components = append(components, &discordgo.ActionsRow{Components: matchButtons[i:end]})
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

func (dc *DiscordContext) parseCustomID(customID string) (action, username, tag string, start, end int, err error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 5 {
		return "", "", "", 0, 0, fmt.Errorf("invalid customID format")
	}
	start, errStart := strconv.Atoi(parts[3])
	end, errEnd := strconv.Atoi(parts[4])
	if errStart != nil || errEnd != nil {
		return "", "", "", 0, 0, fmt.Errorf("error parsing start or end from customID")
	}
	return parts[0], parts[1], parts[2], start, end, nil
}

// Handles interactions with components.
func (dc *DiscordContext) interactionCreateHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		logrus.Infof("Received an interaction of type %d, not a message component interaction", i.Type)
		return
	}

	customID := i.MessageComponentData().CustomID
	action, username, tag, start, end, err := dc.parseCustomID(customID)
	if err != nil {
		logrus.WithError(err).Error("invalid customID")
		dc.respondWithError(s, i.Interaction, "Invalid interaction data.")
		return
	}

	switch {
	case strings.HasPrefix(action, "show_match_"):
		dc.handleShowMatchDetails(s, i, username, tag, start, end, action)
	case action == "go_back":
		dc.handlePageHistory(s, i, username, tag, start, end)
	case action == "show_next_button" || action == "show_previous_button":
		dc.handlePageNavigation(s, i, username, tag, action, start, end)
	default:
		logrus.Errorf("Unrecognized action: %s", action)
		dc.respondWithError(s, i.Interaction, "Unhandled action.")
	}
}

func (dc *DiscordContext) respondWithError(s *discordgo.Session, interaction *discordgo.Interaction, errMsg string) {
	s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: errMsg,
		},
	})
}

func (dc *DiscordContext) respondWithoutError(s *discordgo.Session, interaction *discordgo.Interaction, errMsg string) {
	err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to acknowledge interaction")
		return
	}
}

func (dc *DiscordContext) handleShowMatchDetails(s *discordgo.Session, i *discordgo.InteractionCreate, username, tag string, start, end int, action string) {
	matchIndex, err := strconv.Atoi(action[len("show_match_"):])
	if err != nil {
		logrus.WithError(err).Error("Invalid match index")
		dc.respondWithError(s, i.Interaction, "Invalid match selected.")
		return
	}

	matchDetails, err := dc.handleHistoryRequest(username, tag, start, end)
	if err != nil || matchIndex < 0 || matchIndex >= len(matchDetails) {
		logrus.WithError(err).Error("Failed to get match details")
		dc.respondWithError(s, i.Interaction, "Failed to retrieve match details.")
		return
	}

	// Convert the match details of the selected match to a string description.
	matchDescription := dc.formatMatchDetails([]MatchDetail{matchDetails[matchIndex]})

	// Remove previous components and replace with a "Go Back" button.
	goBackButton := dc.createButton("go_back", "Go Back", "⬅️", username, tag, start, end)

	// Creating a new embed with detailed information.
	embed := discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Match %d Details", matchIndex+1),
		Description: matchDescription,
	}

	// Send an edit to update the message instead of responding to an interaction.
	edit := &discordgo.MessageEdit{
		ID:         i.Message.ID,
		Channel:    i.ChannelID,
		Embeds:     []*discordgo.MessageEmbed{&embed}, // Embed the detailed description.
		Components: []discordgo.MessageComponent{&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&goBackButton}}},
	}

	_, err = s.ChannelMessageEditComplex(edit)
	if err != nil {
		logrus.WithError(err).Error("Failed to edit message for match details")
	} else {
		logrus.Info("Edited the message successfully to show match details")
	}
}

func (dc *DiscordContext) handlePageNavigation(s *discordgo.Session, i *discordgo.InteractionCreate, username, tag string, action string, start, end int) {

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to acknowledge interaction")
		return
	}

	pageChanged := false
	if action == "show_next_button" {
		start = end
		end += 5
		pageChanged = true
	} else if action == "show_previous_button" && start > 0 {
		end = start
		start -= 5
		if start < 0 {
			start = 0
		}
		pageChanged = true
	}

	if !pageChanged {
		return
	}

	matchDetails, err := dc.handleHistoryRequest(username, tag, start, end)
	if err != nil {
		logrus.WithError(err).Error("Failed to get history")
	}
	title := fmt.Sprintf("History (Page %d)", start/5+1)
	description := dc.formatBasicMatchDetails(matchDetails, username, tag)

	embed := discordgo.MessageEmbed{
		Title:       title,
		Description: description,
	}

	nextButton := dc.createButton("show_next_button", "Next", "➡️", username, tag, start, end)
	previousButton := dc.createButton("show_previous_button", "Previous", "⬅️", username, tag, start, end)

	matchButtons := make([]discordgo.MessageComponent, len(matchDetails))
	for i := range matchDetails {
		matchButtons[i] = dc.createButton(fmt.Sprintf("show_match_%d", i), fmt.Sprintf("%d", i+1), "📜", username, tag, start, end)
	}

	components := []discordgo.MessageComponent{
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&previousButton, &nextButton}},
	}

	for i := 0; i < len(matchButtons); i += 5 {
		end := i + 5
		if end > len(matchButtons) {
			end = len(matchButtons)
		}
		components = append(components, &discordgo.ActionsRow{Components: matchButtons[i:end]})
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
}

func (dc *DiscordContext) handlePageHistory(s *discordgo.Session, i *discordgo.InteractionCreate, username, tag string, start, end int) {
	matchDetails, err := dc.handleHistoryRequest(username, tag, start, end)
	if err != nil {
		logrus.WithError(err).Error("Failed to get history")
		return
	}

	title := fmt.Sprintf("History (Page %d)", start/5+1)
	description := dc.formatBasicMatchDetails(matchDetails, username, tag)

	embed := discordgo.MessageEmbed{
		Title:       title,
		Description: description,
	}

	nextButton := dc.createButton("show_next_button", "Next", "➡️", username, tag, start, end)
	previousButton := dc.createButton("show_previous_button", "Previous", "⬅️", username, tag, start, end)

	matchButtons := make([]discordgo.MessageComponent, len(matchDetails))
	for i := range matchDetails {
		matchButtons[i] = dc.createButton(fmt.Sprintf("show_match_%d", i), fmt.Sprintf("%d", i+1), "📜", username, tag, start, end)
	}

	components := []discordgo.MessageComponent{
		&discordgo.ActionsRow{Components: []discordgo.MessageComponent{&previousButton, &nextButton}},
	}

	for i := 0; i < len(matchButtons); i += 5 {
		end := i + 5
		if end > len(matchButtons) {
			end = len(matchButtons)
		}
		components = append(components, &discordgo.ActionsRow{Components: matchButtons[i:end]})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		logrus.WithError(err).Error("Failed to acknowledge interaction")
		return
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
}

func formatDuration(durationSeconds int) string {
	minutes := durationSeconds / 60
	seconds := durationSeconds % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func (dc *DiscordContext) formatMatchDetails(matchDetails []MatchDetail) string {
	var descriptionBuilder strings.Builder
	for _, match := range matchDetails {
		// Determine the winner
		winner := "🔵 Blue Team"
		if match.Winner == "Red Team" {
			winner = "🔴 Red Team"
		}

		// Constructing each match's description.
		fmt.Fprintf(&descriptionBuilder, "VICTORY - %s WINS\nType: %s, Score: %d - %d, Time: %s\n",
			winner, match.GameMode, match.BlueTeam.TotalKills, match.RedTeam.TotalKills,
			formatDuration(match.GameDuration))

		// Formatting details for each team.
		dc.formatTeamDetails(&descriptionBuilder, "🔵 Blue Team", match.BlueTeam, match)
		dc.formatTeamDetails(&descriptionBuilder, "🔴 Red Team", match.RedTeam, match)

		// Append match start timestamp as a readable date string.
		matchStartTime := time.Unix(match.GameStartTimestamp, 0)
		fmt.Fprintf(&descriptionBuilder, "%s\n\n",
			matchStartTime.Format("02/01/2006 15:04"))
	}

	return descriptionBuilder.String()
}

func (dc *DiscordContext) formatTeamDetails(builder *strings.Builder, teamEmoji string, teamDetail TeamDetail, match MatchDetail) {
	fmt.Fprintf(builder, "%s\nTotal Kills: %d\n", teamEmoji, teamDetail.TotalKills)
	for _, participant := range teamDetail.Participants {
		championName := dc.resolveChampionID(participant.ChampionID)
		fmt.Fprintf(builder, "%s#%s - %s %d/%d/%d\nKDA: %.2f, CS: %d (%.2f), DMG: %d, GOLD: %d\n",
			participant.SummonerName, participant.SummonerTag, championName, participant.Kills,
			participant.Deaths, participant.Assists, participant.KDA, participant.CS,
			float64(participant.CS)/float64(match.GameDuration/60), participant.DamageDealt, participant.Gold)
	}
	builder.WriteString("\n")
}

type ChampionData struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type ChampionsResponse map[string]ChampionData

func (dc *DiscordContext) resolveChampionID(championID int) string {
	if name, ok := dc.ChampionData[championID]; ok {
		return name
	}

	resp, err := http.Get("https://cdn.merakianalytics.com/riot/lol/resources/latest/en-US/champions.json")
	if err != nil {
		fmt.Println("Error fetching champion data:", err)
		return "Unknown Champion"
	}
	defer resp.Body.Close()

	var champions ChampionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&champions); err != nil {
		fmt.Println("Error decoding champion data:", err)
		return "Unknown Champion"
	}

	dc.ChampionData = make(map[int]string)
	for _, champion := range champions {
		dc.ChampionData[champion.ID] = champion.Name
	}

	return dc.ChampionData[championID]
}

func (dc *DiscordContext) createButton(action, label, emoji, username, tag string, start, end int) discordgo.Button {
	button := discordgo.Button{
		Label:    label,
		Style:    discordgo.PrimaryButton,
		CustomID: fmt.Sprintf("%s:%s:%s:%d:%d", action, username, tag, start, end),
	}
	if emoji != "" {
		button.Emoji = discordgo.ComponentEmoji{
			Name: emoji,
		}
	}
	return button
}
