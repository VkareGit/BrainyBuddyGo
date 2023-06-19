package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Configuration struct {
	DiscordToken string
	OpenAiToken  string
}

func Load(basepath string) (*Configuration, error) {
	err := godotenv.Load(filepath.Join(basepath, ".env"))

	if err != nil {
		return nil, errors.New("Error loading .env file: " + err.Error())
	}

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return nil, errors.New("DISCORD_BOT_TOKEN not set")
	}

	openAiToken := os.Getenv("OPENAI_API_KEY")
	if openAiToken == "" {
		return nil, errors.New("OPENAI_API_KEY not set")
	}

	return &Configuration{
		DiscordToken: discordToken,
		OpenAiToken:  openAiToken,
	}, nil
}
