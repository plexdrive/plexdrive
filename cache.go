package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"time"

	. "github.com/claudetech/loggo/default"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"golang.org/x/oauth2"
)

// Cache is the cache
type Cache struct {
	db        *gorm.DB
	dbAction  chan cacheAction
	tokenPath string
}

const (
	// StoreAction stores an object in cache
	StoreAction = iota
	// DeleteAction deletes an object in cache
	DeleteAction = iota
)

type cacheAction struct {
	action int
	object *APIObject
}

// APIObject is a Google Drive file object
type APIObject struct {
	ObjectID     string `gorm:"primary_key"`
	Name         string `gorm:"index"`
	IsDir        bool
	Size         uint64
	LastModified time.Time
	DownloadURL  string
	Parents      string `gorm:"index"`
	CreatedAt    time.Time
}

// PageToken is the last change id
type PageToken struct {
	gorm.Model
	Token string
}

// NewCache creates a new cache instance
func NewCache(cacheBasePath string, sqlDebug bool) (*Cache, error) {
	Log.Debugf("Opening cache connection")
	db, err := gorm.Open("sqlite3", filepath.Join(cacheBasePath, "cache"))
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not open cache database")
	}

	Log.Debugf("Migrating cache schema")
	db.AutoMigrate(&APIObject{})
	db.AutoMigrate(&PageToken{})
	db.LogMode(sqlDebug)

	cache := Cache{
		db:        db,
		dbAction:  make(chan cacheAction),
		tokenPath: filepath.Join(cacheBasePath, "token.json"),
	}

	go cache.startStoringQueue()

	return &cache, nil
}

func (c *Cache) startStoringQueue() {
	for {
		action := <-c.dbAction

		if nil != action.object {
			if action.action == DeleteAction || action.action == StoreAction {
				Log.Debugf("Deleting object %v", action.object.ObjectID)
				c.db.Delete(action.object)
			}
			if action.action == StoreAction {
				Log.Debugf("Storing object %v in cache", action.object.ObjectID)
				c.db.Create(action.object)
			}
		}
	}
}

// Close closes all handles
func (c *Cache) Close() error {
	Log.Debugf("Closing cache connection")

	close(c.dbAction)
	if err := c.db.Close(); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not close cache connection")
	}

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

	var object APIObject
	c.db.Where(&APIObject{ObjectID: id}).First(&object)

	Log.Tracef("Got object from cache %v", object)

	if "" != object.ObjectID {
		return &object, nil
	}

	return nil, fmt.Errorf("Could not find object %v in cache", id)
}

// GetObjectsByParent get all objects under parent id
func (c *Cache) GetObjectsByParent(parent string) ([]*APIObject, error) {
	Log.Tracef("Getting children for %v", parent)

	var objects []*APIObject
	c.db.Where("parents LIKE ?", fmt.Sprintf("%%|%v|%%", parent)).Find(&objects)

	Log.Tracef("Got objects from cache %v", objects)

	if 0 != len(objects) {
		return objects, nil
	}

	return nil, fmt.Errorf("Could not find children for parent %v in cache", parent)
}

// GetObjectByParentAndName finds a child element by name and its parent id
func (c *Cache) GetObjectByParentAndName(parent, name string) (*APIObject, error) {
	Log.Tracef("Getting object %v in parent %v", name, parent)

	var object APIObject
	c.db.Where("parents LIKE ? AND name = ?", fmt.Sprintf("%%|%v|%%", parent), name).First(&object)

	Log.Tracef("Got object from cache %v", object)

	if "" != object.ObjectID {
		return &object, nil
	}

	return nil, fmt.Errorf("Could not find object with name %v in parent %v", name, parent)
}

// DeleteObject deletes an object by id
func (c *Cache) DeleteObject(id string) error {
	c.dbAction <- cacheAction{
		action: DeleteAction,
		object: &APIObject{ObjectID: id},
	}
	return nil
}

// UpdateObject updates an object
func (c *Cache) UpdateObject(object *APIObject) error {
	c.dbAction <- cacheAction{
		action: StoreAction,
		object: object,
	}
	return nil
}

// StoreStartPageToken stores the page token for changes
func (c *Cache) StoreStartPageToken(token string) error {
	Log.Debugf("Storing page token %v in cache", token)

	c.db.Delete(&PageToken{})
	c.db.Create(&PageToken{
		Token: token,
	})

	return nil
}

// GetStartPageToken gets the start page token
func (c *Cache) GetStartPageToken() (string, error) {
	Log.Debugf("Getting start page token from cache")

	var pageToken PageToken
	c.db.First(&pageToken)

	Log.Tracef("Got start page token %v", pageToken.Token)

	if "" == pageToken.Token {
		return "", fmt.Errorf("Token not found in cache")
	}

	return pageToken.Token, nil
}
