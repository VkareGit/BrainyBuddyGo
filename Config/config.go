package config

import (
	_ "embed"
)

type Configuration struct {
	DiscordToken string
	OpenAiToken  string
	Production   bool
	OpenAiPrompt []byte
}

var (
	//go:embed discord_bot_token.txt
	discordToken string

	//go:embed open_ai_token.txt
	openAiToken string

	//go:embed prompt.json
	promptData []byte
)

func Load() (*Configuration, error) {
	return &Configuration{
		DiscordToken: discordToken,
		OpenAiToken:  openAiToken,
		Production:   true,
		OpenAiPrompt: promptData,
	}, nil
}
