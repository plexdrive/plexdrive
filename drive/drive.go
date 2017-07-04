package drive

import (
	"context"
	"fmt"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
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

	log.WithField("FirstCheck", firstCheck).Info("Checking for changes")

	client, err := d.getClient()
	if nil != err {
		log.WithField("FirstCheck", firstCheck).
			WithField("Error", err).
			Warning("Could not get Google Drive client to watch for changes")
		return
	}

	// get the last token
	pageToken, err := d.cache.GetStartPageToken()
	if nil != err {
		pageToken = "1"
		log.WithField("FirstCheck", firstCheck).
			WithField("PageToken", pageToken).
			Info("No last change id found, starting from beginning...")
	} else {
		log.WithField("FirstCheck", firstCheck).
			WithField("PageToken", pageToken).
			Debug("Last change id found, continuing getting changes")
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
			log.WithField("FirstCheck", firstCheck).
				WithField("PageToken", pageToken).
				WithField("Error", err).
				Warning("Could not get changes")
			break
		}

		objects := make([]*APIObject, 0)
		for _, change := range results.Changes {
			if change.Removed || (nil != change.File && change.File.ExplicitlyTrashed) {
				if err := d.cache.DeleteObject(change.FileId); nil != err {
					log.WithField("FirstCheck", firstCheck).
						WithField("PageToken", pageToken).
						WithField("Error", err).
						Warning("Could not delete object from cache")
				}
				deletedItems++
			} else {
				object, err := d.mapFileToObject(change.File)
				if nil != err {
					log.WithField("FirstCheck", firstCheck).
						WithField("PageToken", pageToken).
						WithField("ObjectID", change.File.Id).
						WithField("ObjectName", change.File.Name).
						WithField("Error", err).
						Warning("Could not map Google Drive file to object")
				} else {
					objects = append(objects, object)
					updatedItems++
				}
			}

			processedItems++
		}
		if err := d.cache.batchUpdateObjects(objects); nil != err {
			log.WithField("FirstCheck", firstCheck).
				WithField("PageToken", pageToken).
				WithField("Error", err).
				Warning("Could not update objects in cache")
			return
		}

		if processedItems > 0 {
			log.WithField("FirstCheck", firstCheck).
				WithField("PageToken", pageToken).
				WithField("ProcessedItems", processedItems).
				WithField("DeletedItems", deletedItems).
				WithField("UpdatedItems", updatedItems).
				Info("Indexing status")
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

	log.WithField("FirstCheck", firstCheck).
		WithField("PageToken", pageToken).
		Info("Cache build process finished")

	d.changesChecking = false
}

func (d *Client) authorize() error {
	log.Debugf("Authorizing against Google Drive API")

	token, err := d.cache.LoadToken()
	if nil != err {
		log.Debugf("Token could not be found, fetching new one")

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
	log.Debug("Getting root from API")

	client, err := d.getClient()
	if nil != err {
		log.WithField("Error", err).Debug("Could not get Google Drive client")
		return nil, fmt.Errorf("Could not get Google Drive client")
	}

	file, err := client.Files.
		Get(d.rootNodeID).
		Fields(googleapi.Field(Fields)).
		Do()
	if nil != err {
		log.WithField("ObjectID", d.rootNodeID).
			WithField("Error", err).
			Debug("Could not get object from API")
		return nil, fmt.Errorf("Could not get object %v from API", d.rootNodeID)
	}

	// getting file size
	if file.MimeType != "application/vnd.google-apps.folder" && 0 == file.Size {
		res, err := client.Files.Get(d.rootNodeID).Download()
		if nil != err {
			log.WithField("ObjectID", d.rootNodeID).
				WithField("Error", err).
				Debug("Could not get file size for object")
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
		log.WithField("Error", err).Debug("Could not get Google Drive client")
		return fmt.Errorf("Could not get Google Drive client")
	}

	if object.CanTrash {
		if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Trashed: true}).Do(); nil != err {
			log.WithField("ObjectID", object.ObjectID).
				WithField("ObjectName", object.Name).
				WithField("Error", err).
				Debug("Could not delete object from API")
			return fmt.Errorf("Could not delete object %v (%v) from API", object.ObjectID, object.Name)
		}
	} else {
		if _, err := client.Files.Update(object.ObjectID, nil).RemoveParents(parent).Do(); nil != err {
			log.WithField("ObjectID", object.ObjectID).
				WithField("ObjectName", object.Name).
				WithField("Error", err).
				Debug("Could not unsubscribe object from API")
			return fmt.Errorf("Could not unsubscribe object %v (%v) from API", object.ObjectID, object.Name)
		}
	}

	if err := d.cache.DeleteObject(object.ObjectID); nil != err {
		log.WithField("ObjectID", object.ObjectID).
			WithField("ObjectName", object.Name).
			WithField("Error", err).
			Debug("Could not delete object from cache")
		return fmt.Errorf("Could not delete object %v (%v) from cache", object.ObjectID, object.Name)
	}

	return nil
}

// Mkdir creates a new directory in Google Drive
func (d *Client) Mkdir(parent string, name string) (*APIObject, error) {
	client, err := d.getClient()
	if nil != err {
		log.WithField("Error", err).Debug("Could not get Google Drive client")
		return nil, fmt.Errorf("Could not get Google Drive client")
	}

	created, err := client.Files.Create(&gdrive.File{Name: name, Parents: []string{parent}, MimeType: "application/vnd.google-apps.folder"}).Do()
	if nil != err {
		log.WithField("ObjectName", name).
			WithField("Error", err).
			Debug("Could not create object in API")
		return nil, fmt.Errorf("Could not create object (%v) in API", name)
	}

	file, err := client.Files.Get(created.Id).Fields(googleapi.Field(Fields)).Do()
	if nil != err {
		log.WithField("ObjectID", created.Id).
			WithField("ObjectName", name).
			WithField("Error", err).
			Debug("Could not get object fields from API")
		return nil, fmt.Errorf("Could not get object fields %v (%v) from API", created.Id, name)
	}

	obj, err := d.mapFileToObject(file)
	if nil != err {
		log.WithField("ObjectID", file.Id).
			WithField("ObjectName", file.Name).
			WithField("Error", err).
			Debug("Could not map file to object")
		return nil, fmt.Errorf("Could not map file to object %v (%v)", file.Id, file.Name)
	}

	if err := d.cache.UpdateObject(obj); nil != err {
		log.WithField("ObjectID", obj.ObjectID).
			WithField("ObjectName", obj.Name).
			WithField("Error", err).
			Debug("Could not create object in cache")
		return nil, fmt.Errorf("Could not create object %v (%v) in cache", obj.ObjectID, obj.Name)
	}

	return obj, nil
}

// Rename renames file in Google Drive
func (d *Client) Rename(object *APIObject, oldParent string, newParent string, newName string) error {
	client, err := d.getClient()
	if nil != err {
		log.WithField("Error", err).Debug("Could not get Google Drive client")
		return fmt.Errorf("Could not get Google Drive client")
	}

	if _, err := client.Files.Update(object.ObjectID, &gdrive.File{Name: newName}).RemoveParents(oldParent).AddParents(newParent).Do(); nil != err {
		log.WithField("ObjectID", object.ObjectID).
			WithField("ObjectName", object.Name).
			WithField("Error", err).
			Debug("Could not rename object in API")
		return fmt.Errorf("Could not rename object %v (%v) in API", object.ObjectID, object.Name)
	}

	object.Name = newName
	for i, p := range object.Parents {
		if p == oldParent {
			object.Parents = append(object.Parents[:i], object.Parents[i+1:]...)
			break
		}
	}
	object.Parents = append(object.Parents, newParent)

	if err := d.cache.UpdateObject(object); nil != err {
		log.WithField("ObjectID", object.ObjectID).
			WithField("ObjectName", object.Name).
			WithField("Error", err).
			Debug("Could not rename object in cache")
		return fmt.Errorf("Could not rename object %v (%v) in cache", object.ObjectID, object.Name)
	}

	return nil
}

// mapFileToObject maps a Google Drive file to APIObject
func (d *Client) mapFileToObject(file *gdrive.File) (*APIObject, error) {
	lastModified, err := time.Parse(time.RFC3339, file.ModifiedTime)
	if nil != err {
		log.WithField("ObjectID", file.Id).
			WithField("ObjectName", file.Name).
			WithField("Error", err).
			Warning("Could not parse last modified date")
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
