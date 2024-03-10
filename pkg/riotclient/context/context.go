package riotapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/KnutZuidema/golio"
	"github.com/KnutZuidema/golio/api"
	"github.com/sirupsen/logrus"
)

type RiotContext struct {
	Client *golio.Client
	APIKey string
}

type AccountDto struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

func NewRiotAPI(apiKey string) *RiotContext {
	client := golio.NewClient(apiKey, golio.WithRegion(api.RegionEuropeNorthEast), golio.WithLogger(logrus.New()))
	return &RiotContext{
		Client: client,
		APIKey: apiKey,
	}
}

func GetAccountByRiotID(apiKey, region, gameName, tagLine string) (*AccountDto, error) {
	url := fmt.Sprintf("https://%s.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s", region, gameName, tagLine)

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request error: %w", err)
	}
	req.Header.Add("X-Riot-Token", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body) // Read the response body for detailed error message.
		return nil, fmt.Errorf("API request failed with status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body error: %w", err)
	}

	var account AccountDto
	if err := json.Unmarshal(bodyBytes, &account); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	return &account, nil
}

func (r *RiotContext) GetPlayerRankedStats(puuid string) (string, error) {
	logrus.Infof("Fetching ranked stats for summoner puuid: %s", puuid)
	summoner, err := r.Client.Riot.LoL.Summoner.GetByPUUID(puuid)
	if err != nil {
		logrus.WithError(err).Error("Failed to get summoner by name")
		return "", err
	}

	logrus.Infof("Successfully fetched summoner ID for %s: %s", puuid, summoner.ID)
	leagueItems, err := r.Client.Riot.LoL.League.ListBySummoner(summoner.ID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get league entries for summoner")
		return "", err
	}

	var rankedStatsOutput string
	for _, item := range leagueItems {
		rankedStatsOutput += fmt.Sprintf("Queue Type: %s, Rank: %s %s, LP: %d, Wins: %d, Losses: %d\n",
			item.QueueType, item.Tier, item.Rank, item.LeaguePoints, item.Wins, item.Losses)
	}

	logrus.Infof("Successfully fetched ranked stats for %s", puuid)
	logrus.Infof("Ranked stats for %s: %s", puuid, rankedStatsOutput)
	return rankedStatsOutput, nil
}
