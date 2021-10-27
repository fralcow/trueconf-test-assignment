package main

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func parseUserId(r *http.Request) (id uint, err error) {
	stringId := chi.URLParam(r, "id")
	id64, err := strconv.ParseUint(stringId, 10, 32)
	if err != nil {
		return
	}
	return uint(id64), nil
}
