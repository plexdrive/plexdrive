package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// Config describes the basic configuration architecture
type Config struct {
	ClientID     string
	ClientSecret string
}

// ReadConfig reads the configuration based on a filesystem path
func ReadConfig(configPath string) (*Config, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if nil != err {
		return nil, fmt.Errorf("Could not read config file in %v", configPath)
	}

	var config Config
	json.Unmarshal(configFile, &config)
	return &config, nil
}
