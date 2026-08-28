package handler

import "github.com/go-chi/chi/v5"

func Routes(shortenerHandler *ShortenerHandler) chi.Router {
	chiRouter := chi.NewRouter()
	chiRouter.Post("/shorten", shortenerHandler.shorten)
	chiRouter.Get("/links", shortenerHandler.list)
	chiRouter.Get("/{code}", shortenerHandler.resolve)

	return chiRouter
}
