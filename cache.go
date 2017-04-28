package main

import (
	"fmt"

	"strconv"

	"time"

	"github.com/HouzuoGuo/tiedot/db"
	. "github.com/claudetech/loggo/default"
	"golang.org/x/oauth2"
)

// Cache is the cache
type Cache struct {
	db      *db.DB
	objects *db.Col
	tokens  *db.Col
}

// APIObject is a Google Drive file object
type APIObject struct {
}

// NewCache creates a new cache instance
func NewCache(cachePath string) (*Cache, error) {
	Log.Debugf("Opening database connection")
	database, err := db.OpenDB(cachePath)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not open cache database")
	}

	Log.Debugf("Creating object collection in database")
	database.Create("objects")
	Log.Debugf("Creating tokens collection in database")
	database.Create("tokens")

	objectDB := database.Use("objects")
	tokenDB := database.Use("tokens")

	return &Cache{
		db:      database,
		objects: objectDB,
		tokens:  tokenDB,
	}, nil
}

// Close closes all handles
func (c *Cache) Close() error {
	Log.Debugf("Closing database connection")
	if err := c.db.Close(); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not close database connection")
	}

	return nil
}

// LoadToken loads a token from database
func (c *Cache) LoadToken() (*oauth2.Token, error) {
	Log.Debugf("Loading token from database")

	results := make(map[int]struct{})
	if err := db.EvalAllIDs(c.tokens, &results); nil != err {
		Log.Debugf("%v", err)
		return nil, fmt.Errorf("Could not get token id from database")
	}
	Log.Tracef("Got token ids from database %v", results)

	for id := range results {
		r, err := c.tokens.Read(id)
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not read token from database")
		}
		Log.Tracef("Got token result from database %v", r)

		expiry, err := strconv.ParseInt(r["Expiry"].(string), 10, 64)
		if nil != err {
			Log.Debugf("%v", err)
			return nil, fmt.Errorf("Could not parse expiry date")
		}

		return &oauth2.Token{
			AccessToken:  r["AccessToken"].(string),
			Expiry:       time.Unix(expiry, 0),
			RefreshToken: r["RefreshToken"].(string),
			TokenType:    r["TokenType"].(string),
		}, nil
	}

	return nil, fmt.Errorf("Token not found in database")
}

// StoreToken stores a token in the database or updates the existing token element
func (c *Cache) StoreToken(token *oauth2.Token) error {
	Log.Debugf("Storing token to database")

	_, err := c.tokens.Insert(map[string]interface{}{
		"AccessToken":  token.AccessToken,
		"Expiry":       strconv.FormatInt(token.Expiry.Unix(), 10),
		"RefreshToken": token.RefreshToken,
		"TokenType":    token.TokenType,
	})

	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not store token to database")
	}

	return nil
}

// // Close closes the database connection
// func (c *DefaultCache) Close() {
// 	c.db.Close()
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
