package main

import (
	"crypto/sha256"
	"errors"
	"io"
	"log"
	"os"
	path2 "path"
	"sync"
	"time"
)

type ObjectType int
const(
	File ObjectType = iota
	Folder
)

type ObjectInfo struct {
	Type ObjectType `json:"type"`
	Name string `json:"name"`
	Path string `json:"path"`
	LastModified time.Time  `json:"last_modified"`
	Size int64  `json:"size"`
	Hash []byte  `json:"hash"` // In folders, it is the hash of all the hashes concated together!
	SubObjects []*ObjectInfo `json:"sub_objects"`
}

func RecurseDirs(path string) (*ObjectInfo, map[string]*ObjectInfo, error) {
	Entries := make(map[string]*ObjectInfo)
	EntriesLock := sync.RWMutex{}
	dirs, err := recurseDirs(path, nil, nil, &Entries, &EntriesLock)
	if err != nil {
		return nil, nil, err
	}
	return dirs, Entries, err
}

func recurseDirs(path string, routines *map[string]bool, lock *sync.RWMutex, Entries *map[string]*ObjectInfo, EntriesLock *sync.RWMutex) (*ObjectInfo, error) {
	// Get all the objects in the path
	zibi, err := os.ReadDir(path)
	if err != nil {
		if lock != nil && routines != nil {
			lock.Lock()
			(*routines)[path] = false
			lock.Unlock()
		}
		return nil, err
	}
	// List
	var currentEntries []*ObjectInfo
	runningRoutines := make(map[string]bool) // Paths
	RunningRoutinesLock := sync.RWMutex{}
	for _, entry := range zibi  {
		subPath := path2.Join(path, entry.Name())
		if entry.IsDir() {
			RunningRoutinesLock.Lock()
			runningRoutines[subPath] = true
			RunningRoutinesLock.Unlock()
			go func() {
				_, err := recurseDirs(subPath, &runningRoutines, &RunningRoutinesLock, Entries, EntriesLock)
				if err != nil {
					log.Println("Running on subdir '" + subPath + "' failed. Error: " + err.Error())
				}
			}()

		} else{

			size := int64(0)
			lastMod := time.Date(0, 0, 0, 0, 0, 0, 0, time.Local)
			info, err := entry.Info()
			if err == nil {
				size = info.Size()
				lastMod = info.ModTime()
			}else{
				log.Println("Couldn't determine the size of '" + subPath + "'. Error: " + err.Error())
			}

			var hash []byte
			if calculatedHash, err := calculateFileHash(subPath); err == nil {
				hash = calculatedHash
			}else {
				log.Println("Couldn't determine the hash of '" + subPath + "'. Error: " + err.Error())
			}

			oi := &ObjectInfo{
				Type: File,
				Name: entry.Name(),
				Path: subPath,
				Size: size,
				Hash: hash,
				LastModified: lastMod,
			}

			currentEntries = append(currentEntries, oi)

			EntriesLock.Lock()
			(*Entries)[subPath] = oi
			EntriesLock.Unlock()
		}
	}

	// Wait for all the subdirectory scan routines to finish
	finished := false
	for !finished {
		finished = true
		for _,v := range runningRoutines {
			if v {
				finished = false
				break
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Summary
	// Obtain all the entries from the routines
	for k,_ := range runningRoutines {
		EntriesLock.RLock()
		currentEntries = append(currentEntries, (*Entries)[k])
		EntriesLock.RUnlock()
	}

	// Concat all the hashes
	var allHashes []byte
	var totalSize int64
	for _, v := range currentEntries {
		v.Hash = append(allHashes, v.Hash...)
		totalSize += v.Size
	}
	// Calculate the hash of all the sub item's hashes concated together
	hasher := sha256.New()
	hasher.Write(allHashes)
	hash := hasher.Sum(nil)

	oi := &ObjectInfo{
		Type:         Folder,
		Name:         path2.Base(path),
		Path:         path,
		//LastModified: time.Time{},
		Size:         totalSize,
		Hash:         hash,
		SubObjects: currentEntries,
	}

	EntriesLock.Lock()
	(*Entries)[path] = oi
	EntriesLock.Unlock()
	if lock != nil && routines != nil {
		lock.Lock()
		(*routines)[path] = false
		lock.Unlock()
	}
	return oi, err
}

func calculateFileHash(path string) ([]byte, error) {
	hasher := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("Couldn't open file " + path + " for hashing")
	}
	defer f.Close()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, errors.New("Couldn't copy contents of file " + path + " for hashing")
	}
	return hasher.Sum(nil), nil
}