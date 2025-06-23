package discordclient

import (
	riotapi "BrainyBuddyGo/pkg/riotclient/context"
	"fmt"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

type MatchDetail struct {
	MatchID            string
	GameMode           string
	GameDuration       int
	Winner             string
	BlueTeam           TeamDetail
	RedTeam            TeamDetail
	GameStartTimestamp int64
}

type ParticipantDetail struct {
	SummonerName     string
	SummonerTag      string
	ChampionID       int
	Kills            int
	Deaths           int
	Assists          int
	CS               int
	Gold             int
	DamageDealt      int
	LargestMultiKill int
	KDA              float64
}

type TeamDetail struct {
	TotalKills   int
	Participants []ParticipantDetail
}

func calculateKDA(kills, deaths, assists int) float64 {
	if deaths == 0 {
		deaths = 1
	}
	return float64(kills+assists) / float64(deaths)
}

func (dc *DiscordContext) handleHistoryRequest(gameName string, tagLine string, start int, end int) ([]MatchDetail, error) {
	logrus.Infof("Fetching ranked stats for: %s#%s", gameName, tagLine)
	account, err := riotapi.GetAccountByRiotID(dc.RiotContext.APIKey, "Europe", gameName, tagLine)
	if err != nil {
		logrus.WithError(err).Error("Error retrieving account by Riot ID")
		return nil, err
	}
	logrus.Infof("Successfully fetched PUUID for %s#%s: %s", gameName, tagLine, account.Puuid)

	// Get the match list
	matchList, err := dc.RiotContext.GetMatchListByPUUID(account.Puuid, start, end)
	if err != nil {
		return nil, err
	}

	// Create a list of MatchDetail
	var matchDetails []MatchDetail
	for _, match := range matchList {
		blueTeam := TeamDetail{}
		redTeam := TeamDetail{}

		winner := "Unknown"
		if match.Info.Teams[0].Win {
			winner = "Blue Team"
		} else if match.Info.Teams[1].Win {
			winner = "Red Team"
		}

		for _, participant := range match.Info.Participants {
			participantDetail := ParticipantDetail{
				SummonerName:     participant.SummonerName,
				SummonerTag:      participant.RiotIDTagline,
				ChampionID:       participant.ChampionID,
				Kills:            participant.Kills,
				Deaths:           participant.Deaths,
				Assists:          participant.Assists,
				CS:               participant.TotalMinionsKilled,
				Gold:             participant.GoldEarned,
				DamageDealt:      participant.TotalDamageDealtToChampions,
				LargestMultiKill: participant.LargestMultiKill,
				KDA:              calculateKDA(participant.Kills, participant.Deaths, participant.Assists),
			}

			if participant.TeamID == 100 {
				blueTeam.TotalKills += participant.Kills
				blueTeam.Participants = append(blueTeam.Participants, participantDetail)
			} else {
				redTeam.TotalKills += participant.Kills
				redTeam.Participants = append(redTeam.Participants, participantDetail)
			}
		}

		matchDetails = append(matchDetails, MatchDetail{
			MatchID:            match.Metadata.MatchID,
			GameMode:           match.Info.GameMode,
			GameDuration:       match.Info.GameDuration,
			Winner:             winner,
			BlueTeam:           blueTeam,
			RedTeam:            redTeam,
			GameStartTimestamp: match.Info.GameStartTimestamp,
		})
	}

	return matchDetails, nil
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
