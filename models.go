package main

import (
	"net/http"

	"github.com/go-chi/render"
)

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
