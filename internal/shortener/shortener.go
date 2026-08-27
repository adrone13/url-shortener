package shortener

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL = errors.New("invalid url")
	ErrNotFound   = errors.New("short url not found")
)

type Link struct {
	OriginalURL string
	Code        string
}

type Repository interface {
	Save(ctx context.Context, link Link) error
	Get(ctx context.Context, code string) (string, error)
}

type Shortener struct {
	repo Repository
}

func New(repo Repository) *Shortener {
	return &Shortener{repo: repo}
}

func (s *Shortener) Shorten(ctx context.Context, originalURL string) (string, error) {
	u, err := url.Parse(originalURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: missing or unsupported scheme", ErrInvalidURL)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%w: missing host", ErrInvalidURL)
	}

	code := generateCode()

	link := Link{
		OriginalURL: originalURL,
		Code:        code,
	}

	if err := s.repo.Save(ctx, link); err != nil {
		return "", err
	}

	return code, nil
}

func (s *Shortener) Resolve(ctx context.Context, code string) (string, error) {
	originalURL, err := s.repo.Get(ctx, code)
	if err != nil {
		return "", err
	}

	return originalURL, nil
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func generateCode() string {
	var sb strings.Builder
	for range 7 {
		randInt := rand.IntN(len(alphabet))
		sb.WriteByte(alphabet[randInt])
	}

	return sb.String()
}
