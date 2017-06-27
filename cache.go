package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"time"

	"github.com/boltdb/bolt"
	. "github.com/claudetech/loggo/default"
	"golang.org/x/oauth2"
)

// Cache is the cache
type Cache struct {
	db        *bolt.DB
	tokenPath string
}

const (
	// StoreAction stores an object in cache
	StoreAction = iota
	// DeleteAction deletes an object in cache
	DeleteAction = iota
)

type cacheAction struct {
	action  int
	object  *APIObject
	instant bool
}

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
}

// NewCache creates a new cache instance
func NewCache(cacheBasePath string) (*Cache, error) {
	Log.Debugf("Opening cache connection")

	db, err := bolt.Open(filepath.Join(cacheBasePath, "cache.bolt"), 0600, nil)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not open bolt db file")
	}

	cache := Cache{
		db:        db,
		tokenPath: filepath.Join(cacheBasePath, "token.json"),
	}

	// Make sure the necessary buckets exist
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("api_objects")); nil != err {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("idx_api_objects_by_parent")); nil != err {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("page_token")); nil != err {
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
func (c *Cache) GetObject(id string) (*APIObject, error) {
	Log.Tracef("Getting object %v", id)

	var v []byte
	c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("api_objects"))
		v = b.Get([]byte(id))
		return nil
	})
	if nil == v {
		return nil, fmt.Errorf("Could not find object %v in cache", id)
	}

	var object APIObject
	if err := json.Unmarshal(v, &object); nil != err {
		return nil, err
	}

	Log.Tracef("Got object from cache %v", object)
	return &object, nil
}

// GetObjectsByParent get all objects under parent id
func (c *Cache) GetObjectsByParent(parent string) ([]*APIObject, error) {
	Log.Tracef("Getting children for %v", parent)

	objects := make([]*APIObject, 0)
	c.db.View(func(tx *bolt.Tx) error {
		cr := tx.Bucket([]byte("idx_api_objects_by_parent")).Cursor()

		// Iterate over all object ids stored under the parent in the index
		objectIds := make([]string, 0)
		prefix := []byte(parent + "/")
		for k, v := cr.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = cr.Next() {
			objectIds = append(objectIds, string(v))
		}

		// Fetch all objects for the given ids
		b := tx.Bucket([]byte("api_objects"))
		for _, id := range objectIds {
			var object APIObject
			v := b.Get([]byte(id))
			if err := json.Unmarshal(v, &object); nil != err {
				panic(err)
			}
			objects = append(objects, &object)
		}
		return nil
	})

	Log.Tracef("Got objects from cache %v", objects)
	return objects, nil
}

// GetObjectByParentAndName finds a child element by name and its parent id
func (c *Cache) GetObjectByParentAndName(parent, name string) (*APIObject, error) {
	Log.Tracef("Getting object %v in parent %v", name, parent)

	var object *APIObject
	c.db.View(func(tx *bolt.Tx) error {
		// Look up object id in parent-name index
		b := tx.Bucket([]byte("idx_api_objects_by_parent"))
		v := b.Get([]byte(parent + "/" + name))
		if nil == v {
			return nil
		}

		// Fetch object for given id
		id := string(v)
		b = tx.Bucket([]byte("api_objects"))
		v = b.Get([]byte(id))
		if err := json.Unmarshal(v, &object); nil != err {
			panic(err)
		}
		return nil
	})

	if object == nil {
		return nil, fmt.Errorf("Could not find object with name %v in parent %v", name, parent)
	}

	Log.Tracef("Got object from cache %v", object)
	return object, nil
}

// DeleteObject deletes an object by id
func (c *Cache) DeleteObject(id string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("api_objects"))
		v := b.Get([]byte(id))
		if nil == v {
			// No object found in cache? No cleanup necessary
			return nil
		}

		// We need to read the object to cleanup the index
		var object APIObject
		if err := json.Unmarshal(v, &object); nil != err {
			panic(err)
		}
		b.Delete([]byte(id))

		// Remove object ids from the index
		b = tx.Bucket([]byte("idx_api_objects_by_parent"))
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
	if err := c.DeleteObject(object.ObjectID); nil != err {
		return err
	}
	v, err := json.Marshal(object)
	if err != nil {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not marshal object %v (%v)", object.ObjectID, object.Name)
	}

	err = c.db.Update(func(tx *bolt.Tx) error {
		// Store the object in the cache by id
		b := tx.Bucket([]byte("api_objects"))
		if err := b.Put([]byte(object.ObjectID), v); nil != err {
			return err
		}

		// Store the object id by parent-name in the the index
		b = tx.Bucket([]byte("idx_api_objects_by_parent"))
		for _, parent := range object.Parents {
			if err := b.Put([]byte(parent+"/"+object.Name), []byte(object.ObjectID)); nil != err {
				return err
			}
		}
		return nil
	})

	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not update/save object %v (%v)", object.ObjectID, object.Name)
	}

	return nil
}

// StoreStartPageToken stores the page token for changes
func (c *Cache) StoreStartPageToken(token string) error {
	Log.Debugf("Storing page token %v in cache", token)
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("page_token"))
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
		b := tx.Bucket([]byte("page_token"))
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
