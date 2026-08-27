package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/adrone13/url-shortener/internal/shortener"
)

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	ShortURL string `json:"short_url"`
}

type resolveResponse struct {
	URL string `json:"url"`
}

type ShortenerHandler struct {
	logger *slog.Logger
	svc    *shortener.Shortener
}

func NewShortenerHandler(logger *slog.Logger, svc *shortener.Shortener) *ShortenerHandler {
	return &ShortenerHandler{logger, svc}
}

func (sh *ShortenerHandler) shorten(w http.ResponseWriter, r *http.Request) {
	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		sh.logger.Error("failed to decode request", slog.Any("error", err))
		return
	}

	shortUrl, err := sh.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		http.Error(w, "failed to shorten url", http.StatusInternalServerError)
		sh.logger.Error("failed to shorten url", slog.Any("error", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shortenResponse{ShortURL: shortUrl})
}

func (sh *ShortenerHandler) resolve(w http.ResponseWriter, r *http.Request) {
	shortUrl := chi.URLParam(r, "shortUrl")
	if shortUrl == "" {
		http.Error(w, "invalid short url", http.StatusBadRequest)
		sh.logger.Error("invalid short url", slog.String("short_url", shortUrl))
		return
	}

	originalUrl, err := sh.svc.Resolve(r.Context(), shortUrl)
	if err != nil {
		http.Error(w, "failed to resolve url", http.StatusInternalServerError)
		sh.logger.Error("failed to resolve url", slog.Any("error", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resolveResponse{URL: originalUrl})
}
