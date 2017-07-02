package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
)

// Config describes the basic configuration architecture
type Config struct {
	ClientID     string
	ClientSecret string
}

// Read reads the configuration based on a filesystem path
func Read(configPath string) (*Config, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if nil != err {
		return nil, fmt.Errorf("Could not read config file in %v", configPath)
	}

	var config Config
	json.Unmarshal(configFile, &config)
	return &config, nil
}

// CreateConfig creates the configuration by requesting from stdin
func Create(configPath string) (*Config, error) {
	fmt.Println("1. Please go to https://console.developers.google.com/")
	fmt.Println("2. Create a new project")
	fmt.Println("3. Go to library and activate the Google Drive API")
	fmt.Println("4. Go to credentials and create an OAuth client ID")
	fmt.Println("5. Set the application type to 'other'")
	fmt.Println("6. Specify some name and click create")
	fmt.Printf("7. Enter your generated client ID: ")
	var config Config
	if _, err := fmt.Scan(&config.ClientID); err != nil {
		log.Debugf("%v", err)
		return nil, fmt.Errorf("Unable to read client id")
	}
	fmt.Printf("8. Enter your generated client secret: ")
	if _, err := fmt.Scan(&config.ClientSecret); err != nil {
		log.Debugf("%v", err)
		return nil, fmt.Errorf("Unable to read client secret")
	}

	configJSON, err := json.Marshal(&config)
	if nil != err {
		log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not generate config.json content")
	}

	if err := ioutil.WriteFile(configPath, configJSON, 0766); nil != err {
		log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not generate config.json file")
	}

	return &config, nil
}
