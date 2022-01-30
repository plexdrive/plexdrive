package drive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"time"

	. "github.com/claudetech/loggo/default"
	"golang.org/x/oauth2"

	"github.com/boltdb/bolt"
)

// Cache is the cache
type Cache struct {
	db        *bolt.DB
	tokenPath string
}

var (
	bObjects   = []byte("api_objects")
	bParents   = []byte("idx_api_objects_py_parent")
	bPageToken = []byte("page_token")
)

// APIObject is a Google Drive file object
type APIObject struct {
	ObjectID     string
	Name         string
	IsDir        bool
	Size         uint64
	LastModified time.Time
	DownloadURL  string
	Parents      []string
	CanTrash     bool
	MD5Checksum  string
}

// PageToken is the last change id
type PageToken struct {
	ID    string
	Token string
}

// NewCache creates a new cache instance
func NewCache(cacheFile, configPath string, sqlDebug bool) (*Cache, error) {
	Log.Debugf("Opening cache connection")

	db, err := bolt.Open(cacheFile, 0600, nil)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not open cache file")
	}

	cache := Cache{
		db:        db,
		tokenPath: filepath.Join(configPath, "token.json"),
	}

	// Make sure the necessary buckets exist
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bObjects); nil != err {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bParents); nil != err {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bPageToken); nil != err {
			return err
		}
		return nil
	})

	return &cache, err
}

// Close closes all handles
func (c *Cache) Close() error {
	Log.Debugf("Closing cache file")
	c.db.Close()
	return nil
}

// LoadToken loads a token from cache
func (c *Cache) LoadToken() (*oauth2.Token, error) {
	Log.Debugf("Loading token from cache")

	tokenFile, err := ioutil.ReadFile(c.tokenPath)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not read token file in %v", c.tokenPath)
	}

	var token oauth2.Token
	json.Unmarshal(tokenFile, &token)

	Log.Tracef("Got token from cache %v", token)

	return &token, nil
}

// StoreToken stores a token in the cache or updates the existing token element
func (c *Cache) StoreToken(token *oauth2.Token) error {
	Log.Debugf("Storing token to cache")

	tokenJSON, err := json.Marshal(token)
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not generate token.json content")
	}

	if err := ioutil.WriteFile(c.tokenPath, tokenJSON, 0644); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not generate token.json file")
	}

	return nil
}

// GetObject gets an object by id
func (c *Cache) GetObject(id string) (object *APIObject, err error) {
	Log.Tracef("Getting object %v", id)

	c.db.View(func(tx *bolt.Tx) error {
		object, err = boltGetObject(tx, id)
		return nil
	})
	if nil != err {
		return nil, err
	}

	Log.Tracef("Got object from cache %v", object)
	return object, err
}

// GetObjectsByParent get all objects under parent id
func (c *Cache) GetObjectsByParent(parent string) ([]*APIObject, error) {
	Log.Tracef("Getting children for %v", parent)

	objects := make([]*APIObject, 0)
	c.db.View(func(tx *bolt.Tx) error {
		cr := tx.Bucket(bParents).Cursor()

		// Iterate over all object ids stored under the parent in the index
		objectIds := make([]string, 0)
		prefix := []byte(parent + "/")
		for k, v := cr.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = cr.Next() {
			objectIds = append(objectIds, string(v))
		}

		// Fetch all objects for the given ids
		for _, id := range objectIds {
			if object, err := boltGetObject(tx, id); nil == err {
				objects = append(objects, object)
			}
		}
		return nil
	})

	Log.Tracef("Got objects from cache %v", objects)
	return objects, nil
}

// GetObjectByParentAndName finds a child element by name and its parent id
func (c *Cache) GetObjectByParentAndName(parent, name string) (object *APIObject, err error) {
	Log.Tracef("Getting object %v in parent %v", name, parent)

	c.db.View(func(tx *bolt.Tx) error {
		// Look up object id in parent-name index
		b := tx.Bucket(bParents)
		v := b.Get([]byte(parent + "/" + name))
		if nil == v {
			return nil
		}

		// Fetch object for given id
		object, err = boltGetObject(tx, string(v))
		return nil
	})
	if nil != err {
		return nil, err
	}

	if object == nil {
		return nil, fmt.Errorf("Could not find object with name %v in parent %v", name, parent)
	}

	Log.Tracef("Got object from cache %v", object)
	return object, nil
}

// DeleteObject deletes an object by id
func (c *Cache) DeleteObject(id string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bObjects)
		object, _ := boltGetObject(tx, id)
		if nil == object {
			return nil
		}

		b.Delete([]byte(id))

		// Remove object ids from the index
		b = tx.Bucket(bParents)
		for _, parent := range object.Parents {
			b.Delete([]byte(parent + "/" + object.Name))
		}

		return nil
	})
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not delete object %v", id)
	}

	return nil
}

// UpdateObject updates an object
func (c *Cache) UpdateObject(object *APIObject) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		return boltUpdateObject(tx, object)
	})

	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not update/save object %v (%v)", object.ObjectID, object.Name)
	}

	return nil
}

func boltStoreObject(tx *bolt.Tx, object *APIObject) error {
	b := tx.Bucket(bObjects)
	v, err := json.Marshal(object)
	if nil != err {
		return err
	}
	return b.Put([]byte(object.ObjectID), v)
}

func boltGetObject(tx *bolt.Tx, id string) (*APIObject, error) {
	b := tx.Bucket(bObjects)
	v := b.Get([]byte(id))
	if v == nil {
		return nil, fmt.Errorf("Could not find object %v in cache", id)
	}

	var object APIObject
	err := json.Unmarshal(v, &object)
	return &object, err
}

func boltUpdateObject(tx *bolt.Tx, object *APIObject) error {
	prev, _ := boltGetObject(tx, object.ObjectID)
	if nil != prev {
		// Remove object ids from the index
		b := tx.Bucket(bParents)
		for _, parent := range prev.Parents {
			b.Delete([]byte(parent + "/" + prev.Name))
		}
	}

	if err := boltStoreObject(tx, object); nil != err {
		return err
	}

	// Store the object id by parent-name in the index
	b := tx.Bucket(bParents)
	for _, parent := range object.Parents {
		if err := b.Put([]byte(parent+"/"+object.Name), []byte(object.ObjectID)); nil != err {
			return err
		}
	}
	return nil
}

func (c *Cache) BatchUpdateObjects(objects []*APIObject) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		for _, object := range objects {
			if err := boltUpdateObject(tx, object); nil != err {
				return err
			}
		}
		return nil
	})

	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not update/save objects: %v", err)
	}

	return nil
}

// StoreStartPageToken stores the page token for changes
func (c *Cache) StoreStartPageToken(token string) error {
	Log.Debugf("Storing page token %v in cache", token)
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bPageToken)
		return b.Put([]byte("t"), []byte(token))
	})

	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not store token %v", token)
	}

	return nil
}

// GetStartPageToken gets the start page token
func (c *Cache) GetStartPageToken() (string, error) {
	var pageToken string

	Log.Debugf("Getting start page token from cache")
	c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bPageToken)
		v := b.Get([]byte("t"))
		pageToken = string(v)
		return nil
	})
	if pageToken == "" {
		return "", fmt.Errorf("Could not get token from cache, token is empty")
	}

	Log.Tracef("Got start page token %v", pageToken)
	return pageToken, nil
}
