package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/adrone13/url-shortener/internal/shortener"
)

type Repo struct {
	mu      sync.RWMutex
	storage map[string]string
}

func New() *Repo {
	return &Repo{storage: make(map[string]string)}
}

func (mr *Repo) Save(_ context.Context, url shortener.Url) error {
	mr.mu.Lock() // exclusive access for writing
	defer mr.mu.Unlock()

	if _, ok := mr.storage[url.Short]; ok {
		return errors.New("short url already exists")
	}

	mr.storage[url.Short] = url.Original

	return nil
}

func (mr *Repo) Get(_ context.Context, shortUrl string) (string, error) {
	mr.mu.RLock() // multiple readers allowed
	defer mr.mu.RUnlock()

	originalUrl, ok := mr.storage[shortUrl]
	if !ok {
		return "", errors.New("short url not found")
	}

	return originalUrl, nil
}
