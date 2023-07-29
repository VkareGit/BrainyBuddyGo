package config

import (
	_ "embed"
)

type Configuration struct {
	DiscordToken string
	OpenAiToken  string
}

var (
	//go:embed discord_bot_token.txt
	discordToken string

	//go:embed open_ai_token.txt
	openAiToken string
)

func Load() (*Configuration, error) {
	return &Configuration{
		DiscordToken: discordToken,
		OpenAiToken:  openAiToken,
	}, nil
}
