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
	gorm.Model
	ObjectID     string
	Name         string
	IsDir        bool
	Size         uint64
	LastModified time.Time
	DownloadURL  string
	Parents      string
}

// OAuth2Token is the internal gorm structure for the access token
type OAuth2Token struct {
	gorm.Model
	AccessToken  string
	Expiry       time.Time
	RefreshToken string
	TokenType    string
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
	Log.Debugf("Getting object with id %v", id)

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

// func (c *Cache) GetObjectsByParent(parent string, loadFromAPI func(string) ([]*APIObject, error)) ([]*APIObject, error) {
// 	var query interface{}
// 	json.Unmarshal([]byte(fmt.Sprintf(`{"in": ["Parents"], "eq": "%v", "limit": 1}`, id)), &query)
// 	Log.Tracef("Query: %v", query)

// 	ids := make(map[int]struct{})
// 	if err := db.EvalQuery(query, c.objects, &ids); nil != err {
// 		Log.Debugf("%v", err)
// 		return nil, fmt.Errorf("Could not evaluate cache query")
// 	}
// 	Log.Tracef("Got object ids from cache %v", ids)
// }

// // Open a file handle
// func (c *DefaultCache) Open(object *APIObject, chunkSize int64) (*Buffer, error) {
// 	return c.client.Open(object, chunkSize)
// }

// // GetRootID gets the root id
// func (c *DefaultCache) GetRootID() (string, error) {
// 	return "root", nil
// }

// // GetObject gets the file by id
// func (c *DefaultCache) GetObject(id string, forceRefresh bool) (*APIObject, error) {
// 	var object *APIObject
// 	if !forceRefresh {
// 		err := c.db.View(func(tx *bolt.Tx) error {
// 			bucket := tx.Bucket(BUCKET)

// 			val := bucket.Get([]byte(id))
// 			json.Unmarshal(val, &object)

// 			return nil
// 		})

// 		if nil != err {
// 			return nil, err
// 		}
// 	}

// 	if nil == object {
// 		var err error
// 		object, err = c.client.GetObject(id)
// 		if nil != err {
// 			return nil, err
// 		}
// 		err = c.store(object)
// 		if nil != err {
// 			log.Printf("Could not cache object %v", id)
// 		}

// 		if "root" == id {
// 			object.ID = "root"
// 			err = c.store(object)
// 			if nil != err {
// 				log.Printf("Could not cache object root")
// 			}
// 		}
// 	}

// 	return object, nil
// }

// // GetObjectByNameAndParent gets a file by parent id and name
// func (c *DefaultCache) GetObjectByNameAndParent(name, parentID string) (*APIObject, error) {
// 	objects, err := c.GetObjectsByParent(parentID, false)
// 	if nil != err {
// 		return nil, err
// 	}

// 	for _, object := range objects {
// 		if name == object.Name {
// 			return object, nil
// 		}
// 	}

// 	return nil, fmt.Errorf("Could not find %v in %v", name, parentID)
// }

// // GetObjectsByParent get all files by parent id
// func (c *DefaultCache) GetObjectsByParent(parentID string, forceRefresh bool) ([]*APIObject, error) {
// 	var results []*APIObject
// 	var childIds []string
// 	if !forceRefresh {
// 		err := c.db.View(func(tx *bolt.Tx) error {
// 			parent := tx.Bucket(PARENT)

// 			err := json.Unmarshal(parent.Get([]byte(parentID)), &childIds)
// 			if nil != err {
// 				return nil
// 			}

// 			return nil
// 		})

// 		if nil != err {
// 			log.Println(err)
// 		}
// 	}

// 	if len(childIds) > 0 {
// 		for _, id := range childIds {
// 			obj, err := c.GetObject(id, false)
// 			if nil != err {
// 				log.Printf("Could not resolve child id %v\n", id)
// 				continue
// 			}
// 			results = append(results, obj)
// 		}
// 	} else {
// 		var err error
// 		results, err = c.client.GetObjectsByParent(parentID)
// 		if nil != err {
// 			log.Printf("Could not get objects %v", err)
// 		} else {
// 			c.storeChildren(parentID, results)
// 		}
// 	}

// 	return results, nil
// }

// // Store a object and link its parents
// func (c *DefaultCache) Store(object *APIObject) error {
// 	err := c.store(object)
// 	if nil != err {
// 		return err
// 	}

// 	parents := object.Parents

// 	for _, parent := range parents {
// 		o, err := c.GetObject(parent, true)
// 		if nil != err {
// 			return fmt.Errorf("Could not update parent %v of %v", parent, object.ID)
// 		}
// 		_, err = c.GetObjectsByParent(parent, true)
// 		if nil != err {
// 			return fmt.Errorf("Could not refresh children of parent %v", parent)
// 		}
// 		parents = append(parents, o.Parents...)
// 	}

// 	return nil
// }

// func (c *DefaultCache) storeChildren(parentID string, objects []*APIObject) error {
// 	var objIds []string
// 	for _, object := range objects {
// 		objIds = append(objIds, object.ID)
// 	}

// 	return c.db.Update(func(tx *bolt.Tx) error {
// 		parent := tx.Bucket(PARENT)

// 		serialized, err := json.Marshal(objIds)
// 		if nil != err {
// 			return err
// 		}

// 		parent.Put([]byte(parentID), serialized)
// 		return nil
// 	})
// }

// func (c *DefaultCache) store(obj *APIObject) error {
// 	return c.db.Update(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket(BUCKET)

// 		serialized, err := json.Marshal(obj)
// 		if nil != err {
// 			return err
// 		}

// 		objid := []byte(obj.ID)
// 		err = bucket.Put(objid, serialized)

// 		return nil
// 	})
// }
