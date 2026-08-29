package shortener

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	List(ctx context.Context) ([]Link, error)
}

type Cache interface {
	Get(ctx context.Context, code string) (string, bool, error)
	Set(ctx context.Context, code string, originalURL string) error
}

type Shortener struct {
	repo   Repository
	cache  Cache
	logger *slog.Logger
}

func New(repo Repository, cache Cache, logger *slog.Logger) *Shortener {
	return &Shortener{repo, cache, logger}
}

// Shorten
// TODO: add retry on duplicate
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

	if err = s.cache.Set(ctx, code, originalURL); err != nil {
		s.logger.Warn("failed to set cache", slog.Any("error", err))
	}

	return code, nil
}

func (s *Shortener) Resolve(ctx context.Context, code string) (string, error) {
	originalURL, ok, err := s.cache.Get(ctx, code)
	if err != nil {
		s.logger.Warn("failed to get cache", slog.Any("error", err))
	}
	if ok {
		return originalURL, nil
	}

	originalURL, err = s.repo.Get(ctx, code)
	if err != nil {
		return "", err
	}

	if err = s.cache.Set(ctx, code, originalURL); err != nil {
		s.logger.Warn("failed to set cache", slog.Any("error", err))
	}

	return originalURL, nil
}

func (s *Shortener) List(ctx context.Context) ([]Link, error) {
	return s.repo.List(ctx)
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
