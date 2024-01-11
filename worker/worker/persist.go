package worker

import (
	"encoding/gob"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/TUM-Dev/gocast/worker/cfg"
)

var persisted *Persistable

type Deletable struct { // Deletable is a file that can safely be deleted
	File string    // File is the path of the file to delete
	Time time.Time // Time is the time the file was marked for deletion
}

type Persistable struct { // Persistable is a struct for all persistable objects
	Deletable []Deletable // Deletable are all files that can safely be deleted
	mutex     *sync.Mutex
}

const persistFileName = "/persist.gob"

// writeOut writes out the persistable object to disk
func (p *Persistable) writeOut() error {
	f, err := os.OpenFile(cfg.PersistDir+persistFileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	err = gob.NewEncoder(f).Encode(p)
	return err
}

// NewPersistable reads in the persistable object from disk and returns it
func NewPersistable() (*Persistable, error) {
	p := &Persistable{mutex: &sync.Mutex{}}
	f, err := os.Open(cfg.PersistDir + persistFileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// create if not existing
			return p, p.writeOut()
		}
		return nil, err
	}
	defer f.Close()
	err = gob.NewDecoder(f).Decode(p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// AddDeletable adds a file to the list of deletable files
func (p *Persistable) AddDeletable(file string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.Deletable = append(p.Deletable, struct {
		File string
		Time time.Time
	}{
		File: file,
		Time: time.Now(),
	})
	return p.writeOut()
}

// SetDeletables sets the list of deletable files
func (p *Persistable) SetDeletables(d []Deletable) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.Deletable = d
	return p.writeOut()
}
