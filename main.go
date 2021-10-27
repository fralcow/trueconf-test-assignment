package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	log "github.com/sirupsen/logrus"
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

type CreateUserRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

func (c *CreateUserRequest) Bind(r *http.Request) error { return nil }

func createUser(w http.ResponseWriter, r *http.Request) {
	s, err := getUserStore()
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	request := CreateUserRequest{}

	if err := render.Bind(r, &request); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	s.Increment++
	u := User{
		CreatedAt:   time.Now(),
		DisplayName: request.DisplayName,
		Email:       request.Email,
	}

	id := s.Increment
	s.List[id] = u

	b, err := json.Marshal(&s)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	err = ioutil.WriteFile(store, b, fs.ModePerm)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]interface{}{
		"user_id": id,
	})
}

func getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUserId(r)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
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

type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Email       *string `json:"email,omitempty"`
}

func (c *UpdateUserRequest) Bind(r *http.Request) error { return nil }

func updateUser(w http.ResponseWriter, r *http.Request) {
	s, err := getUserStore()
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	request := UpdateUserRequest{}

	if err := render.Bind(r, &request); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	id, err := parseUserId(r)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	if _, ok := s.List[id]; !ok {
		render.Render(w, r, ErrNotFound(UserNotFound))
		return
	}

	u := s.List[id]

	if request.DisplayName != nil {
		u.DisplayName = *request.DisplayName
	}
	if request.Email != nil {
		u.Email = *request.Email
	}

	s.List[id] = u

	b, err := json.Marshal(&s)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}
	err = ioutil.WriteFile(store, b, fs.ModePerm)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	s, err := getUserStore()
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	id, err := parseUserId(r)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	if _, ok := s.List[id]; !ok {
		render.Render(w, r, ErrNotFound(UserNotFound))
		return
	}

	delete(s.List, id)

	b, err := json.Marshal(&s)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	err = ioutil.WriteFile(store, b, fs.ModePerm)
	if err != nil {
		log.Error(err)
		render.Render(w, r, ErrInternal(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

type UserResponse struct {
	*User
	Id uint `json:"id"`
}

func NewUserResponse(id uint) *UserResponse {
	resp := &UserResponse{Id: id}
	if user, _ := dbGetUser(id); user != nil {
		resp.User = user
	}

	return resp
}

func (ur *UserResponse) Render(w http.ResponseWriter, r *http.Request) error {
	if ur.User == nil {
		return UserNotFound
	}
	return nil
}

func NewUsersResopnse(userList *UserList) []render.Renderer {
	list := []render.Renderer{}
	for k := range *userList {
		list = append(list, NewUserResponse(k))
	}
	return list
}
