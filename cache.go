package main

import (
	"fmt"

	"log"

	"encoding/json"

	"github.com/boltdb/bolt"
)

// BUCKET is the default bucket for objects
var BUCKET = []byte("objects")

// PARENT is the parent bucket
var PARENT = []byte("parents")

// Cache is a caching interface for getting files
type Cache interface {
	// GetRootID gets the root id
	GetRootID() (string, error)

	// GetObject gets the file by id
	GetObject(id string, forceRefresh bool) (*APIObject, error)

	// GetObjectByNameAndParent gets a file by parent id and name
	GetObjectByNameAndParent(parentID, name string) (*APIObject, error)

	// GetObjectsByParent get all files by parent id
	GetObjectsByParent(parentID string, forceRefresh bool) ([]*APIObject, error)

	// Close closes the database
	Close()

	// Open a file handle
	Open(object *APIObject, chunkSize int64) (*Buffer, error)

	Store(object *APIObject) error
}

// DefaultCache is the default cache
type DefaultCache struct {
	client APIClient
	db     *bolt.DB
}

// NewDefaultCache creates a new caching instance
func NewDefaultCache(cachePath string, client APIClient) (*DefaultCache, error) {
	db, err := bolt.Open(cachePath, 0644, nil)
	if nil != err {
		return nil, fmt.Errorf("Could not open cache db %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(BUCKET)
		if nil != err {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(PARENT)
		if nil != err {
			return err
		}
		return nil
	})
	if nil != err {
		return nil, fmt.Errorf("Could not create bucket %v", err)
	}

	cache := DefaultCache{
		client: client,
		db:     db,
	}

	return &cache, nil
}

// Close closes the database connection
func (c *DefaultCache) Close() {
	c.db.Close()
}

// Open a file handle
func (c *DefaultCache) Open(object *APIObject, chunkSize int64) (*Buffer, error) {
	return c.client.Open(object, chunkSize)
}

// GetRootID gets the root id
func (c *DefaultCache) GetRootID() (string, error) {
	return "root", nil
}

// GetObject gets the file by id
func (c *DefaultCache) GetObject(id string, forceRefresh bool) (*APIObject, error) {
	var object *APIObject
	if !forceRefresh {
		err := c.db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(BUCKET)

			val := bucket.Get([]byte(id))
			json.Unmarshal(val, &object)

			return nil
		})

		if nil != err {
			return nil, err
		}
	}

	if nil == object {
		var err error
		object, err = c.client.GetObject(id)
		if nil != err {
			return nil, err
		}
		err = c.store(object)
		if nil != err {
			log.Printf("Could not cache object %v", id)
		}

		if "root" == id {
			object.ID = "root"
			err = c.store(object)
			if nil != err {
				log.Printf("Could not cache object root")
			}
		}
	}

	return object, nil
}

// GetObjectByNameAndParent gets a file by parent id and name
func (c *DefaultCache) GetObjectByNameAndParent(name, parentID string) (*APIObject, error) {
	objects, err := c.GetObjectsByParent(parentID, false)
	if nil != err {
		return nil, err
	}

	for _, object := range objects {
		if name == object.Name {
			return object, nil
		}
	}

	return nil, fmt.Errorf("Could not find %v in %v", name, parentID)
}

// GetObjectsByParent get all files by parent id
func (c *DefaultCache) GetObjectsByParent(parentID string, forceRefresh bool) ([]*APIObject, error) {
	var results []*APIObject
	var childIds []string
	if !forceRefresh {
		err := c.db.View(func(tx *bolt.Tx) error {
			parent := tx.Bucket(PARENT)

			err := json.Unmarshal(parent.Get([]byte(parentID)), &childIds)
			if nil != err {
				return nil
			}

			return nil
		})

		if nil != err {
			log.Println(err)
		}
	}

	if len(childIds) > 0 {
		for _, id := range childIds {
			obj, err := c.GetObject(id, false)
			if nil != err {
				log.Printf("Could not resolve child id %v\n", id)
				continue
			}
			results = append(results, obj)
		}
	} else {
		var err error
		results, err = c.client.GetObjectsByParent(parentID)
		if nil != err {
			log.Printf("Could not get objects %v", err)
		} else {
			c.storeChildren(parentID, results)
		}
	}

	return results, nil
}

// Store a object and link its parents
func (c *DefaultCache) Store(object *APIObject) error {
	err := c.store(object)
	if nil != err {
		return err
	}

	parents := object.Parents

	for _, parent := range parents {
		o, err := c.GetObject(parent, true)
		if nil != err {
			return fmt.Errorf("Could not update parent %v of %v", parent, object.ID)
		}
		_, err = c.GetObjectsByParent(parent, true)
		if nil != err {
			return fmt.Errorf("Could not refresh children of parent %v", parent)
		}
		parents = append(parents, o.Parents...)
	}

	return nil
}

func (c *DefaultCache) storeChildren(parentID string, objects []*APIObject) error {
	var objIds []string
	for _, object := range objects {
		objIds = append(objIds, object.ID)
	}

	return c.db.Update(func(tx *bolt.Tx) error {
		parent := tx.Bucket(PARENT)

		serialized, err := json.Marshal(objIds)
		if nil != err {
			return err
		}

		parent.Put([]byte(parentID), serialized)
		return nil
	})
}

func (c *DefaultCache) store(obj *APIObject) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BUCKET)

		serialized, err := json.Marshal(obj)
		if nil != err {
			return err
		}

		objid := []byte(obj.ID)
		err = bucket.Put(objid, serialized)

		return nil
	})
}
