package main

import (
	"fmt"

	"time"

	. "github.com/claudetech/loggo/default"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"golang.org/x/oauth2"
)

// Cache is the cache
type Cache struct {
	db       *gorm.DB
	dbAction chan cacheAction
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

// OAuth2Token is the internal gorm structure for the access token
type OAuth2Token struct {
	gorm.Model
	AccessToken  string
	Expiry       time.Time
	RefreshToken string
	TokenType    string
}

// LargestChangeID is the last change id
type LargestChangeID struct {
	gorm.Model
	ChangeID int64
}

// NewCache creates a new cache instance
func NewCache(cachePath string, sqlDebug bool) (*Cache, error) {
	Log.Debugf("Opening cache connection")
	db, err := gorm.Open("sqlite3", cachePath)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not open cache database")
	}

	Log.Debugf("Migrating cache schema")
	db.AutoMigrate(&OAuth2Token{})
	db.AutoMigrate(&APIObject{})
	db.AutoMigrate(&LargestChangeID{})
	db.LogMode(sqlDebug)

	cache := Cache{
		db:       db,
		dbAction: make(chan cacheAction),
	}

	go cache.startStoringQueue()

	return &cache, nil
}

func (c *Cache) startStoringQueue() {
	for {
		action := <-c.dbAction

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

	var token OAuth2Token
	c.db.First(&token)

	Log.Tracef("Got token from cache %v", token)

	if "" == token.AccessToken {
		return nil, fmt.Errorf("Token not found in cache")
	}

	return &oauth2.Token{
		AccessToken:  token.AccessToken,
		Expiry:       token.Expiry,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
	}, nil
}

// StoreToken stores a token in the cache or updates the existing token element
func (c *Cache) StoreToken(token *oauth2.Token) error {
	Log.Debugf("Storing token to cache")

	c.db.Delete(&OAuth2Token{})
	t := OAuth2Token{
		AccessToken:  token.AccessToken,
		Expiry:       token.Expiry,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
	}

	c.db.Create(&t)

	return nil
}

// GetObject gets an object by id
func (c *Cache) GetObject(id string) (*APIObject, error) {
	Log.Debugf("Getting object %v", id)

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
	Log.Debugf("Getting children for %v", parent)

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
	Log.Debugf("Getting object %v in parent %v", name, parent)

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

// StoreLargestChangeID stores the largest change id
func (c *Cache) StoreLargestChangeID(changeID int64) error {
	Log.Debugf("Storing change id %v in cache", changeID)

	c.db.Delete(&LargestChangeID{})
	c.db.Create(&LargestChangeID{
		ChangeID: changeID,
	})

	return nil
}

// GetLargestChangeID gets the largest change id or zero change id
func (c *Cache) GetLargestChangeID() (int64, error) {
	Log.Debugf("Getting change id from cache")

	var changeID LargestChangeID
	c.db.First(&changeID)

	Log.Tracef("Got change id %v", changeID.ChangeID)

	return changeID.ChangeID, nil
}
