package main

import (
	"fmt"

	"log"

	"encoding/json"

	"io"

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
	GetObject(id string) (*APIObject, error)

	// GetObjectByNameAndParent gets a file by parent id and name
	GetObjectByNameAndParent(parentID, name string) (*APIObject, error)

	// GetObjectsByParent get all files by parent id
	GetObjectsByParent(parentID string) ([]*APIObject, error)

	// Close closes the database
	Close()

	// Download opens the file handle
	Download(id string) (io.ReadCloser, error)
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

// Download opens the file handle
func (c *DefaultCache) Download(id string) (io.ReadCloser, error) {
	return c.client.Download(id)
}

// GetRootID gets the root id
func (c *DefaultCache) GetRootID() (string, error) {
	return "root", nil
}

// GetObject gets the file by id
func (c *DefaultCache) GetObject(id string) (*APIObject, error) {
	var object *APIObject
	err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BUCKET)

		val := bucket.Get([]byte(id))
		json.Unmarshal(val, &object)

		return nil
	})

	if nil != err {
		return nil, err
	}

	if nil == object {
		log.Printf("Could not find object %v in cache", id)
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
	objects, err := c.GetObjectsByParent(parentID)
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
func (c *DefaultCache) GetObjectsByParent(parentID string) ([]*APIObject, error) {
	var results []*APIObject
	var childIds []string
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

	if len(childIds) > 0 {
		for _, id := range childIds {
			obj, err := c.GetObject(id)
			if nil != err {
				log.Printf("Could not resolve child id %v\n", id)
				continue
			}
			results = append(results, obj)
		}
	} else {
		log.Printf("Could not find children for %v in cache", parentID)
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

func (c *DefaultCache) storeChildren(parentID string, objects []*APIObject) error {
	log.Printf("Caching children for %v", parentID)

	var objIds []string
	for _, object := range objects {
		objIds = append(objIds, object.ID)
	}

	log.Printf("Storing children ids: %v", objIds)

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
	log.Printf("Caching object %v (%v)", obj.ID, obj)

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
