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

func (mr *Repo) Save(_ context.Context, link shortener.Link) error {
	mr.mu.Lock() // exclusive access for writing
	defer mr.mu.Unlock()

	if _, ok := mr.storage[link.Code]; ok {
		return errors.New("short url already exists")
	}

	mr.storage[link.Code] = link.OriginalURL

	return nil
}

func (mr *Repo) Get(_ context.Context, code string) (string, error) {
	mr.mu.RLock() // multiple readers allowed
	defer mr.mu.RUnlock()

	originalURL, ok := mr.storage[code]
	if !ok {
		return "", shortener.ErrNotFound
	}

	return originalURL, nil
}
