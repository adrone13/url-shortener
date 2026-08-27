package handler

import "github.com/go-chi/chi/v5"

func Routes(shortenerHandler *ShortenerHandler) chi.Router {
	chiRouter := chi.NewRouter()
	chiRouter.Post("/shorten", shortenerHandler.shorten)
	chiRouter.Get("/{shortUrl}", shortenerHandler.resolve)

	return chiRouter
}
