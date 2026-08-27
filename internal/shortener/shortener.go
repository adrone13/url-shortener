package shortener

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/url"
	"strings"
)

type Url struct {
	Original string
	Short    string
}

type UrlStorage interface {
	Save(ctx context.Context, url Url) error
	Get(ctx context.Context, shortUrl string) (string, error)
}

type Shortener struct {
	repo UrlStorage
}

func New(repo UrlStorage) *Shortener {
	return &Shortener{repo: repo}
}

func (s *Shortener) Shorten(ctx context.Context, originalUrl string) (string, error) {
	u, err := url.Parse(originalUrl)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("invalid scheme")
	}
	if u.Host == "" {
		return "", errors.New("invalid host")
	}

	shortUrl := generateShortUrl()

	storedUrl := Url{
		Original: originalUrl,
		Short:    shortUrl,
	}

	if err := s.repo.Save(ctx, storedUrl); err != nil {
		return "", err
	}

	return shortUrl, nil
}

func (s *Shortener) Resolve(ctx context.Context, shortUrl string) (string, error) {
	originalUrl, err := s.repo.Get(ctx, shortUrl)
	if err != nil {
		return "", err
	}

	return originalUrl, nil
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func generateShortUrl() string {
	var sb strings.Builder
	for range 7 {
		randInt := rand.IntN(len(alphabet))
		sb.WriteByte(alphabet[randInt])
	}

	return sb.String()
}
