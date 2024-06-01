package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nDmitry/tgfeed/internal/entity"
)

func Read(configPath string) (*entity.Config, error) {
	contents, err := os.ReadFile(configPath)

	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var config entity.Config

	if err = json.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	return &config, nil
}
