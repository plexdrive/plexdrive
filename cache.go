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
	db *gorm.DB
}

// APIObject is a Google Drive file object
type APIObject struct {
	ObjectID     string `gorm:"primary_key"`
	Name         string
	IsDir        bool
	Size         uint64
	LastModified time.Time
	DownloadURL  string
	Parents      string
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
func NewCache(cachePath string) (*Cache, error) {
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

	return &Cache{
		db: db,
	}, nil
}

// Close closes all handles
func (c *Cache) Close() error {
	Log.Debugf("Closing cache connection")
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
func (c *Cache) GetObject(id string, loadFromAPI func(string) (APIObject, error)) (APIObject, error) {
	Log.Debugf("Getting object %v", id)

	var object APIObject
	c.db.Where(&APIObject{ObjectID: id}).First(&object)

	Log.Tracef("Got object from cache %v", object)

	if "" != object.ObjectID {
		return object, nil
	}

	Log.Debugf("Could not find object %v in cache, loading from API", id)
	o, err := loadFromAPI(id)
	if nil != err {
		Log.Debugf("%v", err)
		return APIObject{}, fmt.Errorf("Could not load object %v from API", id)
	}

	// do not cache root object
	if "root" != id {
		Log.Debugf("Storing object %v in cache", id)
		c.db.Create(&o)
	}
	return o, nil
}

// GetObjectsByParent get all objects under parent id
func (c *Cache) GetObjectsByParent(parent string, loadFromAPI func(string) ([]APIObject, error)) ([]APIObject, error) {
	Log.Debugf("Getting children for %v", parent)

	var objects []APIObject
	c.db.Where("parents LIKE ?", fmt.Sprintf("%%|%v|%%", parent)).Find(&objects)

	Log.Tracef("Got objects from cache %v", objects)

	if 0 != len(objects) {
		return objects, nil
	}

	Log.Debugf("Could not find children for parent %v in cache, loading from API", parent)
	o, err := loadFromAPI(parent)
	if nil != err {
		Log.Debugf("%v", err)
		return []APIObject{}, fmt.Errorf("Could not load children for %v from API", parent)
	}

	for _, object := range o {
		Log.Debugf("Storing object %v in cache", object.ObjectID)
		c.db.Create(&object)
	}

	return o, nil
}

// DeleteObject deletes an object by id
func (c *Cache) DeleteObject(id string) error {
	Log.Debugf("Deleting object %v", id)

	c.db.Where(&APIObject{ObjectID: id}).Delete(&APIObject{})

	return nil
}

// UpdateObject updates an object
func (c *Cache) UpdateObject(object *APIObject) (bool, error) {

	var obj APIObject
	c.db.Where(&APIObject{ObjectID: object.ObjectID}).First(&obj)

	created := false
	if "" == obj.ObjectID {
		Log.Debugf("Storing object %v in cache", object.ObjectID)
		c.db.Create(&object)
		created = true
	} else {
		Log.Debugf("Updating object %v in cache", object.ObjectID)
		c.db.Model(obj).Update(object)
	}

	return created, nil
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
