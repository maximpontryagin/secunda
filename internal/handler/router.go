package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(auth *Auth, teams *Teams, tasks *Tasks, verifier TokenVerifier) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})
	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", auth.Register)
		r.Post("/login", auth.Login)
		r.Group(func(r chi.Router) {
			r.Use(authenticate(verifier))
			r.Post("/teams", teams.Create)
			r.Get("/teams", teams.List)
			r.Post("/teams/{id}/invite", teams.Invite)
			r.Post("/tasks", tasks.Create)
			r.Get("/tasks", tasks.List)
			r.Put("/tasks/{id}", tasks.Update)
			r.Get("/tasks/{id}/history", tasks.History)
			r.Post("/tasks/{id}/comments", tasks.AddComment)
			r.Get("/tasks/{id}/comments", tasks.Comments)
		})
	})
	return router
}
