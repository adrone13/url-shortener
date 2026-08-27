package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/adrone13/url-shortener/internal/shortener"
)

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code string `json:"code"`
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

	code, err := sh.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		if errors.Is(err, shortener.ErrInvalidURL) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "failed to shorten url", http.StatusInternalServerError)
		sh.logger.Error("failed to shorten url", slog.Any("error", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(shortenResponse{Code: code})
}

func (sh *ShortenerHandler) resolve(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, "invalid short url", http.StatusBadRequest)
		sh.logger.Error("invalid short url", slog.String("code", code))
		return
	}

	originalURL, err := sh.svc.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, shortener.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "failed to resolve url", http.StatusInternalServerError)
		sh.logger.Error("failed to resolve url", slog.Any("error", err))
		return
	}

	http.Redirect(w, r, originalURL, http.StatusFound)
}
