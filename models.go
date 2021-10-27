package main

import (
	"net/http"

	"github.com/go-chi/render"
)

type CreateUserRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

func (c *CreateUserRequest) Bind(r *http.Request) error { return nil }

type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Email       *string `json:"email,omitempty"`
}

func (c *UpdateUserRequest) Bind(r *http.Request) error { return nil }

type UserResponse struct {
	*User
	Id uint `json:"id"`
}

func (ur *UserResponse) Render(w http.ResponseWriter, r *http.Request) error {
	if ur.User == nil {
		return UserNotFound
	}
	return nil
}

func NewUserResponse(id uint) *UserResponse {
	resp := &UserResponse{Id: id}
	if user, _ := dbGetUser(id); user != nil {
		resp.User = user
	}

	return resp
}

func NewUsersResopnse(userList *UserList) []render.Renderer {
	list := []render.Renderer{}
	for k := range *userList {
		list = append(list, NewUserResponse(k))
	}
	return list
}
