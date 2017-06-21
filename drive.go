package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"strings"

	. "github.com/claudetech/loggo/default"
	"golang.org/x/oauth2"
	gdrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// Fields are the fields that should be returned by the Google Drive API
var Fields string

// init initializes the global configurations
func init() {
	Fields = "id, name, mimeType, modifiedTime, size, explicitlyTrashed, parents, capabilities/canTrash"
}

// Drive holds the Google Drive API connection(s)
type Drive struct {
	cache           *Cache
	context         context.Context
	token           *oauth2.Token
	config          *oauth2.Config
	rootNodeID      string
	changesChecking bool
}

// NewDriveClient creates a new Google Drive client
func NewDriveClient(config *Config, cache *Cache, refreshInterval time.Duration, rootNodeID string) (*Drive, error) {
	drive := Drive{
		cache:   cache,
		context: context.Background(),
		config: &oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://accounts.google.com/o/oauth2/token",
			},
			RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
			Scopes:      []string{gdrive.DriveScope},
		},
		rootNodeID:      rootNodeID,
		changesChecking: false,
	}

	if "" == drive.rootNodeID {
		drive.rootNodeID = "root"
	}

	if err := drive.authorize(); nil != err {
		return nil, err
	}

	go drive.startWatchChanges(refreshInterval)

	return &drive, nil
}

func (d *Drive) startWatchChanges(refreshInterval time.Duration) {
	d.checkChanges(true)
	for _ = range time.Tick(refreshInterval) {
		d.checkChanges(false)
	}
}

func (d *Drive) checkChanges(firstCheck bool) {
	if d.changesChecking {
		return
	}
	d.changesChecking = true

	Log.Debugf("Checking for changes")

	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		Log.Warningf("Could not get Google Drive client to watch for changes")
		return
	}

	// get the last token
	pageToken, err := d.cache.GetStartPageToken()
	if nil != err {
		pageToken = "1"
		Log.Info("No last change id found, starting from beginning...")
	} else {
		Log.Debugf("Last change id found, continuing getting changes (%v)", pageToken)
	}

	if firstCheck {
		Log.Infof("First cache build process started...")
	}

	deletedItems := 0
	updatedItems := 0
	processedItems := 0
	for {
		query := client.Changes.
			List(pageToken).
			Fields(googleapi.Field(fmt.Sprintf("nextPageToken, newStartPageToken, changes(removed, fileId, file(%v))", Fields))).
			PageSize(1000)

		results, err := query.Do()
		if nil != err {
			Log.Debugf("%v", err)
			Log.Warningf("Could not get changes")
			break
		}

		for _, change := range results.Changes {
			Log.Tracef("Change %v", change)

			if change.Removed || (nil != change.File && change.File.ExplicitlyTrashed) {
				if err := d.cache.DeleteObject(change.FileId); nil != err {
					Log.Tracef("%v", err)
				}
				deletedItems++
			} else {
				object, err := d.mapFileToObject(change.File)
				if nil != err {
					Log.Debugf("%v", err)
					Log.Warningf("Could not map Google Drive file %v (%v) to object", change.File.Id, change.File.Name)
				} else {
					if err := d.cache.UpdateObject(object); nil != err {
						Log.Warningf("%v", err)
					}
					updatedItems++
				}
			}

			processedItems++
		}

		if processedItems > 0 {
			Log.Infof("Processed %v items / deleted %v items / updated %v items",
				processedItems, deletedItems, updatedItems)
		}

		if "" != results.NextPageToken {
			pageToken = results.NextPageToken
			d.cache.StoreStartPageToken(pageToken)
		} else {
			pageToken = results.NewStartPageToken
			d.cache.StoreStartPageToken(pageToken)
			break
		}
	}

	if firstCheck {
		Log.Infof("First cache build process finished!")
	}

	d.changesChecking = false
}

func (d *Drive) authorize() error {
	Log.Debugf("Authorizing against Google Drive API")

	token, err := d.cache.LoadToken()
	if nil != err {
		Log.Debugf("Token could not be found, fetching new one")

		t, err := getTokenFromWeb(d.config)
		if nil != err {
			return err
		}
		token = t
		if err := d.cache.StoreToken(token); nil != err {
			return err
		}
	}

	d.token = token
	return nil
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser %v\n", authURL)
	fmt.Printf("Paste the authorization code: ")

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve token from web %v", err)
	}
	return tok, err
}

// getClient gets a new Google Drive client
func (d *Drive) getClient() (*gdrive.Service, error) {
	return gdrive.New(d.config.Client(d.context, d.token))
}

// getNativeClient gets a native http client
func (d *Drive) getNativeClient() *http.Client {
	return oauth2.NewClient(d.context, d.config.TokenSource(d.context, d.token))
}

// GetRoot gets the root node directly from the API
func (d *Drive) GetRoot() (*APIObject, error) {
	Log.Debugf("Getting root from API")

	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get Google Drive client")
	}

	file, err := client.Files.
		Get(d.rootNodeID).
		Fields(googleapi.Field(Fields)).
		Do()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get object %v from API", d.rootNodeID)
	}

	// getting file size
	if file.MimeType != "application/vnd.google-apps.folder" && 0 == file.Size {
		res, err := client.Files.Get(d.rootNodeID).Download()
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not get file size for object %v", d.rootNodeID)
		}
		file.Size = res.ContentLength
	}

	return d.mapFileToObject(file)
}

// GetObject gets an object by id
func (d *Drive) GetObject(id string) (*APIObject, error) {
	return d.cache.GetObject(id)
}

// GetObjectsByParent get all objects under parent id
func (d *Drive) GetObjectsByParent(parent string) ([]*APIObject, error) {
	return d.cache.GetObjectsByParent(parent)
}

// GetObjectByParentAndName finds a child element by name and its parent id
func (d *Drive) GetObjectByParentAndName(parent, name string) (*APIObject, error) {
	return d.cache.GetObjectByParentAndName(parent, name)
}

// Open a file
func (d *Drive) Open(object *APIObject) (*Buffer, error) {
	nativeClient := d.getNativeClient()
	return GetBufferInstance(nativeClient, object)
}

// Remove removes file from Google Drive
func (d *Drive) Remove(object *APIObject, parent string) error {
	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not get Google Drive client")
	}

	if object.CanTrash {
		if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Trashed: true}).Do(); nil != err {
			Log.Debugf("%v", err)
			return fmt.Errorf("Could not delete object %v (%v) from API", object.ObjectID, object.Name)
		}
	} else {
		if _, err := client.Files.Update(object.ObjectID, nil).RemoveParents(parent).Do(); nil != err {
			Log.Debugf("%v", err)
			return fmt.Errorf("Could not unsubscribe object %v (%v) from API", object.ObjectID, object.Name)
		}
	}

	if err := d.cache.DeleteObject(object.ObjectID); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not delete object %v (%v) from cache", object.ObjectID, object.Name)
	}

	return nil
}

// Rename renames file in Google Drive
func (d *Drive) Rename(object *APIObject, parent string, NewName string) error {
	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not get Google Drive client")
	}

	p := strings.Join(object.Parents, ",")

	if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Name: NewName}).RemoveParents(p).AddParents(parent).Do(); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not rename object %v (%v) from API", object.ObjectID, object.Name)
	}

	object.Name = NewName
	object.Parents = []string{parent}

	if err := d.cache.UpdateObject(object); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not rename object %v (%v) from cache", object.ObjectID, object.Name)
	}

	return nil
}

// mapFileToObject maps a Google Drive file to APIObject
func (d *Drive) mapFileToObject(file *gdrive.File) (*APIObject, error) {
	Log.Tracef("Converting Google Drive file: %v", file)

	lastModified, err := time.Parse(time.RFC3339, file.ModifiedTime)
	if nil != err {
		Log.Debugf("%v", err)
		Log.Warningf("Could not parse last modified date for object %v (%v)", file.Id, file.Name)
		lastModified = time.Now()
	}

	var parents []string
	for _, parent := range file.Parents {
		parents = append(parents, parent)
	}

	return &APIObject{
		ObjectID:     file.Id,
		Name:         file.Name,
		IsDir:        file.MimeType == "application/vnd.google-apps.folder",
		LastModified: lastModified,
		Size:         uint64(file.Size),
		DownloadURL:  fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%v?alt=media", file.Id),
		Parents:      parents,
		CanTrash:     file.Capabilities.CanTrash,
	}, nil
}
