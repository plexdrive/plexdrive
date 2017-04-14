package plexdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"sh0k.de/plexdrive/config"
)

// Drive holds the Google Drive API connection(s)
type Drive struct {
	context         context.Context
	activeAccountID int
	accounts        []config.Account
	rootDir         string
	tokens          []oauth2.Token
	configs         []oauth2.Config
}

// New creates a new Google Drive instance
func New(accounts []config.Account, tokenPath string, dir string) (*Drive, error) {
	drive := Drive{
		activeAccountID: 1,
		context:         context.Background(),
		accounts:        accounts,
		rootDir:         dir,
	}

	if err := drive.authorize(tokenPath); nil != err {
		return nil, err
	}

	return &drive, nil
}

func (d *Drive) getClient() (*drive.Service, error) {
	client := d.configs[d.activeAccountID-1].Client(d.context, &d.tokens[d.activeAccountID-1])

	// TODO: increase account id only on error
	// if (d.activeAccountID + 1) > len(d.configs) {
	// 	d.activeAccountID = 1
	// } else {
	// 	d.activeAccountID++
	// }
	return drive.New(client)
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
				Scopes:      []string{drive.DriveScope},
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
				Scopes:      []string{drive.DriveScope},
			}
			d.configs = append(d.configs, config)
		}
	}

	return nil
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

// GetRootID gets the ID of the root folder
func (d *Drive) GetRootID() (string, error) {
	if d.rootDir == "/" {
		return "root", nil
	}

	client, err := d.getClient()
	if nil != err {
		return "", err
	}

	r, err := client.Files.List().Q("'root' in parents AND trashed = false").Do()
	if nil != err {
		return "", err
	}

	for _, f := range r.Files {
		if d.rootDir == f.Name {
			return f.Id, nil
		}
	}
	return "", fmt.Errorf("Could not find %s in root directory", d.rootDir)
}

// GetFilesIn gets all files under a parent folder
func (d *Drive) GetFilesIn(parent string) ([]*drive.File, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	var files []*drive.File
	pageToken := ""
	for {
		query := client.Files.List().Q(fmt.Sprintf("'%v' in parents AND trashed = false", parent))

		if "" != pageToken {
			query = query.PageToken(pageToken)
		}

		r, err := query.Do()
		if nil != err {
			return nil, err
		}

		files = append(files, r.Files...)
		pageToken = r.NextPageToken

		if "" == pageToken {
			break
		}
	}

	return files, nil
}

// GetFile gets a file
func (d *Drive) GetFile(fileID string) (*drive.File, error) {
	client, err := d.getClient()
	if nil != err {
		return nil, err
	}

	return client.Files.Get(fileID).Do()
}

// GetFileByNameAndParent gets a file
func (d *Drive) GetFileByNameAndParent(name, parent string) (*drive.File, error) {
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
