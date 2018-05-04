package drive

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/claudetech/loggo/default"
	"github.com/dweidenfeld/plexdrive/config"
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

// Client holds the Google Drive API connection(s)
type Client struct {
	cache           *Cache
	context         context.Context
	token           *oauth2.Token
	config          *oauth2.Config
	rootNodeID      string
	changesChecking bool
}

// NewClient creates a new Google Drive client
func NewClient(config *config.Config, cache *Cache, refreshInterval time.Duration, rootNodeID string) (*Client, error) {
	client := Client{
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

	if "" == client.rootNodeID {
		client.rootNodeID = "root"
	}

	if err := client.authorize(); nil != err {
		return nil, err
	}

	go client.startWatchChanges(refreshInterval)

	return &client, nil
}

func (d *Client) startWatchChanges(refreshInterval time.Duration) {
	d.checkChanges(true)
	for _ = range time.Tick(refreshInterval) {
		d.checkChanges(false)
	}
}

func (d *Client) checkChanges(firstCheck bool) {
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
			PageSize(1000).
			SupportsTeamDrives(true).
			IncludeTeamDriveItems(true)

		results, err := query.Do()
		if nil != err {
			Log.Debugf("%v", err)
			Log.Warningf("Could not get changes")
			break
		}

		objects := make([]*APIObject, 0)
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
					objects = append(objects, object)
					updatedItems++
				}
			}

			processedItems++
		}
		if err := d.cache.BatchUpdateObjects(objects); nil != err {
			Log.Warningf("%v", err)
			return
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

func (d *Client) authorize() error {
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
func (d *Client) getClient() (*gdrive.Service, error) {
	return gdrive.New(d.config.Client(d.context, d.token))
}

// GetNativeClient gets a native http client
func (d *Client) GetNativeClient() *http.Client {
	return oauth2.NewClient(d.context, d.config.TokenSource(d.context, d.token))
}

// GetRoot gets the root node directly from the API
func (d *Client) GetRoot() (*APIObject, error) {
	Log.Debugf("Getting root from API")

	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get Google Drive client")
	}

	file, err := client.Files.
		Get(d.rootNodeID).
		Fields(googleapi.Field(Fields)).
		SupportsTeamDrives(true).
		Do()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get object %v from API", d.rootNodeID)
	}

	// getting file size
	if file.MimeType != "application/vnd.google-apps.folder" && 0 == file.Size {
		res, err := client.Files.Get(d.rootNodeID).SupportsTeamDrives(true).Download()
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not get file size for object %v", d.rootNodeID)
		}
		file.Size = res.ContentLength
	}

	return d.mapFileToObject(file)
}

// GetObject gets an object by id
func (d *Client) GetObject(id string) (*APIObject, error) {
	return d.cache.GetObject(id)
}

// GetObjectsByParent get all objects under parent id
func (d *Client) GetObjectsByParent(parent string) ([]*APIObject, error) {
	return d.cache.GetObjectsByParent(parent)
}

// GetObjectByParentAndName finds a child element by name and its parent id
func (d *Client) GetObjectByParentAndName(parent, name string) (*APIObject, error) {
	return d.cache.GetObjectByParentAndName(parent, name)
}

// Remove removes file from Google Drive
func (d *Client) Remove(object *APIObject, parent string) error {
	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not get Google Drive client")
	}

	if err := d.cache.DeleteObject(object.ObjectID); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not delete object %v (%v) from cache", object.ObjectID, object.Name)
	}

	go func() {
		if object.CanTrash {
			if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Trashed: true}).SupportsTeamDrives(true).Do(); nil != err {
				Log.Debugf("%v", err)
				Log.Warningf("Could not delete object %v (%v) from API", object.ObjectID, object.Name)
				d.cache.UpdateObject(object)
			}
		} else {
			if _, err := client.Files.Update(object.ObjectID, nil).RemoveParents(parent).SupportsTeamDrives(true).Do(); nil != err {
				Log.Debugf("%v", err)
				Log.Warningf("Could not unsubscribe object %v (%v) from API", object.ObjectID, object.Name)
				d.cache.UpdateObject(object)
			}
		}
	}()

	return nil
}

// Mkdir creates a new directory in Google Drive
func (d *Client) Mkdir(parent string, Name string) (*APIObject, error) {
	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get Google Drive client")
	}

	created, err := client.Files.Create(&gdrive.File{Name: Name, Parents: []string{parent}, MimeType: "application/vnd.google-apps.folder"}).SupportsTeamDrives(true).Do()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not create object(%v) from API", Name)
	}

	file, err := client.Files.Get(created.Id).Fields(googleapi.Field(Fields)).SupportsTeamDrives(true).Do()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get object fields %v from API", created.Id)
	}

	Obj, err := d.mapFileToObject(file)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not map file to object %v (%v)", file.Id, file.Name)
	}

	if err := d.cache.UpdateObject(Obj); nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not create object %v (%v) from cache", Obj.ObjectID, Obj.Name)
	}

	return Obj, nil
}

// Rename renames file in Google Drive
func (d *Client) Rename(object *APIObject, OldParent string, NewParent string, NewName string) error {
	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not get Google Drive client")
	}

	if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Name: NewName}).RemoveParents(OldParent).AddParents(NewParent).SupportsTeamDrives(true).Do(); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not rename object %v (%v) from API", object.ObjectID, object.Name)
	}

	object.Name = NewName
	for i, p := range object.Parents {
		if p == OldParent {
			object.Parents = append(object.Parents[:i], object.Parents[i+1:]...)
			break
		}
	}
	object.Parents = append(object.Parents, NewParent)

	if err := d.cache.UpdateObject(object); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not rename object %v (%v) from cache", object.ObjectID, object.Name)
	}

	return nil
}

// mapFileToObject maps a Google Drive file to APIObject
func (d *Client) mapFileToObject(file *gdrive.File) (*APIObject, error) {
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
