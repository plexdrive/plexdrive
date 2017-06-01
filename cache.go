package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"time"

	. "github.com/claudetech/loggo/default"
	"golang.org/x/oauth2"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Cache is the cache
type Cache struct {
	session   *mgo.Session
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
	ObjectID     string `bson:"_id,omitempty"`
	Name         string
	IsDir        bool
	Size         uint64
	LastModified time.Time
	DownloadURL  string
	Parents      []string
	CanTrash     bool
}

// PageToken is the last change id
type PageToken struct {
	ID    string `bson:"_id,omitempty"`
	Token string
}

// NewCache creates a new cache instance
func NewCache(mongoURL string, cacheBasePath string, sqlDebug bool) (*Cache, error) {
	Log.Debugf("Opening cache connection")

	session, err := mgo.Dial(mongoURL)
	if nil != err {
		Log.Debugf("%v")
		return nil, fmt.Errorf("Could not open mongo db connection")
	}

	cache := Cache{
		session:   session,
		tokenPath: filepath.Join(cacheBasePath, "token.json"),
	}

	// create index
	db := session.DB("plexdrive").C("api_objects")
	db.EnsureIndex(mgo.Index{Key: []string{"parents"}})
	db.EnsureIndex(mgo.Index{Key: []string{"name"}})

	return &cache, nil
}

// Close closes all handles
func (c *Cache) Close() error {
	Log.Debugf("Closing cache connection")
	c.session.Close()
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
	db := c.session.DB("plexdrive").C("api_objects")

	var object APIObject
	if err := db.Find(bson.M{"_id": id}).One(&object); nil != err {
		Log.Debugf("GetObject %v", err)
		return nil, fmt.Errorf("Could not find object %v in cache", id)
	}

	Log.Tracef("Got object from cache %v", object)
	return &object, nil
}

// GetObjectsByParent get all objects under parent id
func (c *Cache) GetObjectsByParent(parent string) ([]*APIObject, error) {
	Log.Tracef("Getting children for %v", parent)
	db := c.session.DB("plexdrive").C("api_objects")

	var objects []*APIObject
	if err := db.Find(bson.M{"parents": parent}).All(&objects); nil != err {
		Log.Debugf("GetObjectsByParent %v", err)
		return nil, fmt.Errorf("Could not find children for parent %v in cache", parent)
	}

	Log.Tracef("Got objects from cache %v", objects)
	return objects, nil
}

// GetObjectByParentAndName finds a child element by name and its parent id
func (c *Cache) GetObjectByParentAndName(parent, name string) (*APIObject, error) {
	Log.Tracef("Getting object %v in parent %v", name, parent)
	db := c.session.DB("plexdrive").C("api_objects")

	var object APIObject
	if err := db.Find(bson.M{"parents": parent, "name": name}).One(&object); nil != err {
		Log.Tracef("GetObjectByParentAndName %v", err)
		return nil, fmt.Errorf("Could not find object with name %v in parent %v", name, parent)
	}

	Log.Tracef("Got object from cache %v", object)
	return &object, nil
}

// DeleteObject deletes an object by id
func (c *Cache) DeleteObject(id string) error {
	db := c.session.DB("plexdrive").C("api_objects")

	if err := db.Remove(bson.M{"_id": id}); nil != err {
		Log.Debugf("DeleteObject %v", err)
		return fmt.Errorf("Could not delete object %v", id)
	}

	return nil
}

// UpdateObject updates an object
func (c *Cache) UpdateObject(object *APIObject) error {
	db := c.session.DB("plexdrive").C("api_objects")

	if _, err := db.Upsert(bson.M{"_id": object.ObjectID}, object); nil != err {
		Log.Debugf("UpdateObject %v", err)
		return fmt.Errorf("Could not update/save object %v", object.ObjectID)
	}

	return nil
}

// StoreStartPageToken stores the page token for changes
func (c *Cache) StoreStartPageToken(token string) error {
	Log.Debugf("Storing page token %v in cache", token)
	db := c.session.DB("plexdrive").C("page_token")

	if _, err := db.Upsert(bson.M{"_id": "t"}, &PageToken{ID: "t", Token: token}); nil != err {
		Log.Debugf("StoreStartPageToken %v", err)
		return fmt.Errorf("Could not store token %v", token)
	}

	return nil
}

// GetStartPageToken gets the start page token
func (c *Cache) GetStartPageToken() (string, error) {
	Log.Debugf("Getting start page token from cache")
	db := c.session.DB("plexdrive").C("page_token")

	var pageToken PageToken
	if err := db.Find(nil).One(&pageToken); nil != err {
		Log.Debugf("GetStartPageToken %v", err)
		return "", fmt.Errorf("Could not get token from cache")
	}

	Log.Tracef("Got start page token %v", pageToken.Token)
	return pageToken.Token, nil
}
