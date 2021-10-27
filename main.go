package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	setRoutes(r)

	http.ListenAndServe(":3333", r)
}

func setRoutes(r *chi.Mux) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(time.Now().String()))
	})

	r.Route("/api", func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Route("/users", func(r chi.Router) {
				r.Get("/", searchUsers)
				r.Post("/", createUser)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", getUser)
					r.Patch("/", updateUser)
					r.Delete("/", deleteUser)
				})
			})
		})
	})
	return
}

func searchUsers(w http.ResponseWriter, r *http.Request) {
	userList, err := dbGetUserList()
	if err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}

	if err := render.RenderList(w, r, NewUsersResopnse(userList)); err != nil {
		render.Render(w, r, ErrRender(err))
		return
	}
}

func createUser(w http.ResponseWriter, r *http.Request) {
	request := CreateUserRequest{}
	if err := render.Bind(r, &request); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	id, err := dbCreateUser(request.DisplayName, request.Email)
	if err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, NewUserResponse(id))
}

func getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserId(r)
	if err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}

	if err := render.Render(w, r, NewUserResponse(id)); err != nil {
		if errors.Is(err, UserNotFound) {
			render.Render(w, r, ErrNotFound(err))
			return
		}

		render.Render(w, r, ErrRender(err))
		return
	}
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	request := UpdateUserRequest{}
	if err := render.Bind(r, &request); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	id, err := parseUserId(r)
	if err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}

	if err := dbUpdateUser(id, request.DisplayName, request.Email); err != nil {
		if errors.Is(err, UserNotFound) {
			render.Render(w, r, ErrNotFound(err))
			return
		}

		render.Render(w, r, ErrInternal(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserId(r)
	if err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}

	if err := dbDeleteUser(id); err != nil {
		if errors.Is(err, UserNotFound) {
			render.Render(w, r, ErrNotFound(err))
			return
		}

		render.Render(w, r, ErrInternal(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}
