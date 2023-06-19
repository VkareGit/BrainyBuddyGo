package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Configuration struct {
	DiscordToken string
	OpenAiToken  string
	Production   bool
}

func Load(basepath string) (*Configuration, error) {
	err := godotenv.Load(filepath.Join(basepath, ".env"))

	if err != nil {
		return nil, fmt.Errorf("Error loading .env file: %w", err)
	}

	discordToken, err := getEnvVariable("DISCORD_BOT_TOKEN")
	if err != nil {
		return nil, err
	}

	openAiToken, err := getEnvVariable("OPENAI_API_KEY")
	if err != nil {
		return nil, err
	}

	production := false
	productionEnv := os.Getenv("PRODUCTION")
	if productionEnv == "true" {
		production = true
	}

	return &Configuration{
		DiscordToken: discordToken,
		OpenAiToken:  openAiToken,
		Production:   production,
	}, nil
}

func getEnvVariable(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s not set", key)
	}
	return value, nil
}
