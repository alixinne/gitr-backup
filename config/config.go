package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	//"gopkg.in/yaml.v3"
)

type Config struct {
	Hosts []Host `yaml:"hosts"`
}

func (config *Config) massageConfig() error {
	for i := range config.Hosts {
		err := (&config.Hosts[i]).massageConfig(i)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to parse host %d config", i)
			return err
		}
	}

	return nil
}

type Host struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	BaseUrl string `yaml:"base"`
	Token   string `yaml:"token"`
	Usage   string `yaml:"use_as"`
}

func readEnvVar(logger zerolog.Logger, val *string) error {
	if strings.HasPrefix(*val, "$") {
		name := strings.TrimPrefix(*val, "$")
		value, exists := os.LookupEnv(name)
		if exists {
			logger.Debug().Msgf("Looked up value from %s", *val)
			*val = value
		} else {
			return errors.New(fmt.Sprintf("Missing environment variable %s", *val))
		}
	}

	return nil
}

func (host *Host) massageConfig(i int) error {
	logger := log.With().Int("host", i).Logger()

	if host.Type == "" {
		return errors.New(fmt.Sprintf("Missing host type: %s", host.Type))
	}

	if host.Type != "github" && host.Type != "gitea" {
		return errors.New(fmt.Sprintf("Invalid host type: %s", host.Type))
	}

	if host.Type == "github" {
		if host.BaseUrl == "" {
			host.BaseUrl = "https://github.com"
		}
	} else if host.Type == "gitea" {
		if host.BaseUrl == "" {
			return errors.New("A base url is required for a gitea host")
		}
	}

	if host.Name == "" {
		logger.Info().Msgf("Defaulted name to type (%s)", host.Type)
		host.Name = host.Type
	}

	if host.Token == "" {
		return errors.New("Missing token for authentication")
	}

	err := readEnvVar(logger, &host.Token)
	if err != nil {
		return err
	}

	err = readEnvVar(logger, &host.BaseUrl)
	if err != nil {
		return err
	}

	return nil
}

func LoadConfig(path string) (*Config, error) {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(raw, &config)
	if err != nil {
		return nil, err
	}

	err = config.massageConfig()
	if err != nil {
		return nil, err
	}

	return &config, nil
}
