package config

import (
	"errors"
	"os"
)

var (
	ErrMissingAPIKey = errors.New("DD_API_KEY is required")
	ErrMissingAppKey = errors.New("DD_APP_KEY is required")
)

type Config struct {
	APIKey string
	AppKey string
	Site   string
}

func LoadConfig() (Config, error) {
	apiKey := os.Getenv("DD_API_KEY")
	if apiKey == "" {
		return Config{}, ErrMissingAPIKey
	}

	appKey := os.Getenv("DD_APP_KEY")
	if appKey == "" {
		return Config{}, ErrMissingAppKey
	}

	site := os.Getenv("DD_SITE")
	if site == "" {
		site = "datadoghq.com"
	}

	return Config{
		APIKey: apiKey,
		AppKey: appKey,
		Site:   site,
	}, nil
}
