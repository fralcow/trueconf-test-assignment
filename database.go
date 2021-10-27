package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

const store = `users.json`

type (
	User struct {
		CreatedAt   time.Time `json:"created_at"`
		DisplayName string    `json:"display_name"`
		Email       string    `json:"email"`
	}
	UserList  map[string]User
	UserStore struct {
		Increment int      `json:"increment"`
		List      UserList `json:"list"`
	}
)

func getUserStore() (us UserStore, err error) {

	f, err := ioutil.ReadFile(store)
	if err != nil {
		return
	}

	err = json.Unmarshal(f, &us)
	if err != nil {
		log.Error(err)
		return
	}

	return
}

func overwriteUserStore(us UserStore) (err error) {
	f, err := os.OpenFile(store, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	dat, err := json.Marshal(us)
	if err != nil {
		return
	}
	_, err = f.Write(dat)
	log.Debugf("UserStore data: %v", string(dat))
	return
}
