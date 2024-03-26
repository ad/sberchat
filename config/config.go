package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}

func GetConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(".", "sberchat.json")
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join("config", "sberchat.json")
			if _, err := os.Stat(path); err != nil {
				path = filepath.Join("~", "sberchat.json")
				if _, err := os.Stat(path); err != nil {
					path = ""
				}
			}
		}
	}

	if path == "" {
		return nil, fmt.Errorf("path not found")
	}

	config := &Config{}

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return config, nil

}
