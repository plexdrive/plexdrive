package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	gdrive "google.golang.org/api/drive/v3"

	"golang.org/x/oauth2"
)

// Drive holds the Google Drive API connection(s)
type Drive struct {
	context         context.Context
	activeAccountID int
	accounts        []Account
	tokens          []oauth2.Token
	configs         []oauth2.Config
}

// NewDriveClient creates a new Google Drive instance
func NewDriveClient(accounts []Account, tokenPath string) (*Drive, error) {
	drive := Drive{
		activeAccountID: 1,
		context:         context.Background(),
		accounts:        accounts,
	}

	if err := drive.authorize(tokenPath); nil != err {
		return nil, err
	}

	return &drive, nil
}

// Download opens the file handle
func (d *Drive) Download(id string) (io.ReadCloser, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	httpResponse, err := client.Files.Get(id).Download()
	if nil != err {
		return nil, err
	}

	if httpResponse.StatusCode == 200 {
		return httpResponse.Body, nil
	}

	return nil, fmt.Errorf("Invalid status code %v", httpResponse.StatusCode)
}

// ReadAll reads the whole content of a file
func (d *Drive) ReadAll(id string) ([]byte, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	httpResponse, err := client.Files.Get(id).Download()
	if nil != err {
		return nil, err
	}

	log.Printf("HTTPResponse: %v", httpResponse)

	content, err := ioutil.ReadAll(httpResponse.Body)
	if nil != err {
		return nil, err
	}

	log.Printf("Content: %v", content)

	return content, nil
}

// GetObject gets one object by id
func (d *Drive) GetObject(id string) (*APIObject, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	o, err := client.Files.Get(id).Do()
	if nil != err {
		return nil, err
	}
	return mapDriveToAPIObject(o), nil
}

// GetObjectsByParent gets all files under a parent folder
func (d *Drive) GetObjectsByParent(parentID string) ([]*APIObject, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	var files []*APIObject
	pageToken := ""
	for {
		query := client.Files.List().Q(fmt.Sprintf("'%v' in parents AND trashed = false", parentID))

		if "" != pageToken {
			query = query.PageToken(pageToken)
		}

		r, err := query.Do()
		if nil != err {
			break
		}

		for _, file := range r.Files {
			files = append(files, mapDriveToAPIObject(file))
		}
		pageToken = r.NextPageToken

		if "" == pageToken {
			break
		}
	}

	return files, nil
}

// GetFileByNameAndParent gets a file
func (d *Drive) GetFileByNameAndParent(name, parent string) (*gdrive.File, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	r, err := client.Files.List().Q(fmt.Sprintf("'%v' in parents AND name = '%v' AND trashed = false", parent, name)).Do()
	if nil != err {
		return nil, err
	}

	for _, f := range r.Files {
		if name == f.Name {
			return f, nil
		}
	}
	return nil, fmt.Errorf("Could not find %s in directory %v", name, parent)
}

func (d *Drive) authorize(tokenPath string) error {
	d.tokens = getTokens(tokenPath)
	if len(d.tokens) < len(d.accounts) {
		for _, account := range d.accounts {
			config := oauth2.Config{
				ClientID:     account.ClientID,
				ClientSecret: account.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://accounts.google.com/o/oauth2/auth",
					TokenURL: "https://accounts.google.com/o/oauth2/token",
				},
				RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
				Scopes:      []string{gdrive.DriveScope},
			}
			token := getTokenFromWeb(&config)
			d.configs = append(d.configs, config)
			d.tokens = append(d.tokens, *token)
		}
		if err := storeTokens(tokenPath, d.tokens); nil != err {
			return err
		}
	} else {
		for _, account := range d.accounts {
			config := oauth2.Config{
				ClientID:     account.ClientID,
				ClientSecret: account.ClientSecret,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://accounts.google.com/o/oauth2/auth",
					TokenURL: "https://accounts.google.com/o/oauth2/token",
				},
				RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
				Scopes:      []string{gdrive.DriveScope},
			}
			d.configs = append(d.configs, config)
		}
	}

	return nil
}

func (d *Drive) getClient() (*gdrive.Service, error) {
	client := d.configs[d.activeAccountID-1].Client(d.context, &d.tokens[d.activeAccountID-1])

	// TODO: increase account id only on error
	// if (d.activeAccountID + 1) > len(d.configs) {
	// 	d.activeAccountID = 1
	// } else {
	// 	d.activeAccountID++
	// }
	return gdrive.New(client)
}

func getTokens(tokenPath string) []oauth2.Token {
	var tokens []oauth2.Token
	tokenFile, err := ioutil.ReadFile(tokenPath)
	if nil != err {
		return tokens
	}
	json.Unmarshal(tokenFile, &tokens)
	return tokens
}

func storeTokens(tokenPath string, tokens []oauth2.Token) error {
	j, err := json.Marshal(tokens)
	if nil != err {
		return fmt.Errorf("Could not store tokens, %v", err)
	}
	ioutil.WriteFile(tokenPath, j, 0644)
	return nil
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser %v\n", authURL)
	fmt.Printf("Paste the authorization code: ")

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func mapDriveToAPIObject(file *gdrive.File) *APIObject {
	mtime, err := time.Parse(time.RFC3339, file.ModifiedTime)
	if nil != err {
		mtime = time.Now()
	}

	return &APIObject{
		ID:      file.Id,
		Parents: file.Parents,
		Name:    file.Name,
		IsDir:   file.MimeType == "application/vnd.google-apps.folder",
		Size:    uint64(file.Size),
		MTime:   mtime,
	}
}
