package shortener

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/url"
	"strings"
)

type URL struct {
	Original string
	Short    string
}

type URLStorage interface {
	Save(ctx context.Context, url URL) error
	Get(ctx context.Context, shortURL string) (string, error)
}

type Shortener struct {
	repo URLStorage
}

func New(repo URLStorage) *Shortener {
	return &Shortener{repo: repo}
}

func (s *Shortener) Shorten(ctx context.Context, originalURL string) (string, error) {
	u, err := url.Parse(originalURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("invalid scheme")
	}
	if u.Host == "" {
		return "", errors.New("invalid host")
	}

	shortURL := generateShortURL()

	storedURL := URL{
		Original: originalURL,
		Short:    shortURL,
	}

	if err := s.repo.Save(ctx, storedURL); err != nil {
		return "", err
	}

	return shortURL, nil
}

func (s *Shortener) Resolve(ctx context.Context, shortURL string) (string, error) {
	originalURL, err := s.repo.Get(ctx, shortURL)
	if err != nil {
		return "", err
	}

	return originalURL, nil
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func generateShortURL() string {
	var sb strings.Builder
	for range 7 {
		randInt := rand.IntN(len(alphabet))
		sb.WriteByte(alphabet[randInt])
	}

	return sb.String()
}
