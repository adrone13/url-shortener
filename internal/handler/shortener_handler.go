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

type listResponse struct {
	Links []shortener.Link `json:"links"`
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

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		sh.logger.Error("failed to decode request", slog.Any("error", err))
		return
	}

	sh.logger.Info("shorten request received", slog.String("url", req.URL), slog.Any("body", req))

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
	sh.logger.Info("resolve request received")

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

func (sh *ShortenerHandler) list(w http.ResponseWriter, r *http.Request) {
	sh.logger.Info("list request received")

	links, err := sh.svc.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list links", http.StatusInternalServerError)
		sh.logger.Error("failed to list links", slog.Any("error", err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(listResponse{Links: links})
}
