package drive

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	. "github.com/claudetech/loggo/default"
	"github.com/plexdrive/plexdrive/config"
	"golang.org/x/oauth2"
	gdrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// fields are the fields that should be returned by the Google Drive API
const fields = "id, name, mimeType, modifiedTime, md5Checksum, size, explicitlyTrashed, parents, capabilities/canTrash, shortcutDetails"

// folderMimeType is the mime type of a Google Drive folder
const folderMimeType = "application/vnd.google-apps.folder"

// shortcutMimeType is the mime type of a Google Drive shortcut
const shortcutMimeType = "application/vnd.google-apps.shortcut"

// Client holds the Google Drive API connection(s)
type Client struct {
	cache           *Cache
	context         context.Context
	token           *oauth2.Token
	config          *oauth2.Config
	rootNodeID      string
	driveID         string
	changesChecking bool
	lock            sync.Mutex
}

// NewClient creates a new Google Drive client
func NewClient(config *config.Config, cache *Cache, refreshInterval time.Duration, rootNodeID string, driveID string) (*Client, error) {
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
		rootNodeID: rootNodeID,
		driveID:    driveID,
	}

	if "" == client.rootNodeID {
		client.rootNodeID = "root"
	}
	if "" != client.driveID && client.rootNodeID == "root" {
		client.rootNodeID = client.driveID
	}

	if err := client.authorize(); nil != err {
		return nil, err
	}

	go client.startWatchChanges(refreshInterval)

	return &client, nil
}

func (d *Client) startWatchChanges(refreshInterval time.Duration) {
	d.checkChanges(true)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sig := <-sigChan:
			if sig != syscall.SIGHUP {
				return
			}
			d.checkChanges(false)
		case <-time.After(refreshInterval):
			d.checkChanges(false)
		}
	}
}

func (d *Client) checkChanges(firstCheck bool) {
	d.lock.Lock()
	if d.changesChecking {
		return
	}
	d.changesChecking = true
	d.lock.Unlock()
	defer func() {
		d.lock.Lock()
		d.changesChecking = false
		d.lock.Unlock()
	}()

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
			Fields(googleapi.Field(fmt.Sprintf("nextPageToken, newStartPageToken, changes(changeType, removed, fileId, file(%v))", fields))).
			PageSize(1000).
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			IncludeCorpusRemovals(true)

		if d.driveID != "" {
			query = query.DriveId(d.driveID)
		}

		results, err := query.Do()
		if nil != err {
			Log.Debugf("%v", err)
			Log.Warningf("Could not get changes")
			break
		}

		objects := make([]*APIObject, 0)
		for _, change := range results.Changes {
			Log.Tracef("Change %v", change)
			// ignore changes for changeType drive
			if change.ChangeType != "file" {
				Log.Warningf("Ignoring change type %v", change.ChangeType)
				continue
			}

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
			if pageToken != results.NewStartPageToken {
				pageToken = results.NewStartPageToken
				d.cache.StoreStartPageToken(pageToken)
			} else {
				Log.Debugf("No changes")
			}
			break
		}
	}

	if firstCheck {
		Log.Infof("First cache build process finished!")
	}
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

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve token from web %v", err)
	}
	return tok, err
}

// getClient gets a new Google Drive client
func (d *Client) getClient() (*gdrive.Service, error) {
	return gdrive.NewService(d.context, option.WithHTTPClient(d.config.Client(d.context, d.token)))
}

// GetNativeClient gets a native http client
func (d *Client) GetNativeClient() *http.Client {
	return oauth2.NewClient(d.context, d.config.TokenSource(d.context, d.token))
}

// GetFileById gets a Google Drive file by its id
func (d *Client) GetFileById(id string) (*gdrive.File, error) {
	client, err := d.getClient()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get Google Drive client")
	}

	file, err := client.Files.
		Get(id).
		Fields(fields).
		SupportsAllDrives(true).
		Do()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get object %v from API", id)
	}

	// getting file size
	if 0 == file.Size && folderMimeType != file.MimeType && shortcutMimeType != file.MimeType {
		res, err := client.Files.Get(id).SupportsAllDrives(true).Download()
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not get file size for object %v", id)
		}
		file.Size = res.ContentLength
	}

	return file, nil
}

// GetRoot gets the root node directly from the API
func (d *Client) GetRoot() (*APIObject, error) {
	Log.Debugf("Getting root from API")

	file, err := d.GetFileById(d.rootNodeID)
	if err != nil {
		return nil, err
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
			if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Trashed: true}).SupportsAllDrives(true).Do(); nil != err {
				Log.Debugf("%v", err)
				Log.Warningf("Could not delete object %v (%v) from API", object.ObjectID, object.Name)
				d.cache.UpdateObject(object)
			}
		} else {
			if _, err := client.Files.Update(object.ObjectID, nil).RemoveParents(parent).SupportsAllDrives(true).Do(); nil != err {
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

	created, err := client.Files.Create(&gdrive.File{Name: Name, Parents: []string{parent}, MimeType: folderMimeType}).SupportsAllDrives(true).Do()
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not create object(%v) from API", Name)
	}

	file, err := client.Files.Get(created.Id).Fields(googleapi.Field(fields)).SupportsAllDrives(true).Do()
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

	if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Name: NewName}).RemoveParents(OldParent).AddParents(NewParent).SupportsAllDrives(true).Do(); nil != err {
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

	var err error
	targetFile := file
	if file.MimeType == shortcutMimeType && file.ShortcutDetails != nil {
		targetFile, err = d.GetFileById(file.ShortcutDetails.TargetId)
		if err != nil {
			return nil, err
		}
	}

	lastModified, err := time.Parse(time.RFC3339, targetFile.ModifiedTime)
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
		IsDir:        targetFile.MimeType == folderMimeType,
		LastModified: lastModified,
		Size:         uint64(targetFile.Size),
		DownloadURL:  fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%v?alt=media", targetFile.Id),
		Parents:      parents,
		CanTrash:     file.Capabilities.CanTrash,
		MD5Checksum:  targetFile.Md5Checksum,
	}, err
}
