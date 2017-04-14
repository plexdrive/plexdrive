package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// Config describes the basic configuration architecture
type Config struct {
	Accounts []Account
}

// Account represents one Google Drive account
type Account struct {
	ClientID     string
	ClientSecret string
}

// ReadConfig reads the configuration based on a filesystem path
func ReadConfig(configPath string) *Config {
	configFile, err := ioutil.ReadFile(configPath)
	if nil != err {
		fmt.Printf("Could not read config file on %v\n", configPath)
		os.Exit(1)
	}

	var config Config
	json.Unmarshal(configFile, &config)
	return &config
}
