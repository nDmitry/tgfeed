package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

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

	for _, sw := range config.StopWords {
		config.StopWordsRegexps = append(config.StopWordsRegexps, *regexp.MustCompile("(?i)" + sw))
	}

	return &config, nil
}
