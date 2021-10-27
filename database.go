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
	UserList  map[uint]User
	UserStore struct {
		Increment uint     `json:"increment"`
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

func dbGetUser(id uint) (user *User, err error) {
	s, err := getUserStore()
	if err != nil {
		return
	}

	for k, v := range s.List {
		if k == id {
			return &v, nil
		}
	}
	return nil, UserNotFound
}

func dbGetUserList() (userList *UserList, err error) {
	s, err := getUserStore()
	if err != nil {
		return
	}

	return &s.List, nil
}

func dbCreateUser(displayName, email string) (id uint, err error) {

	s, err := getUserStore()
	if err != nil {
		return
	}

	s.Increment++
	u := User{
		CreatedAt:   time.Now(),
		DisplayName: displayName,
		Email:       email,
	}

	id = s.Increment
	s.List[id] = u

	err = overwriteUserStore(s)
	if err != nil {
		return
	}

	return
}

func dbUpdateUser(id uint, displayName *string, email *string) (err error) {
	us, err := getUserStore()
	if err != nil {
		return
	}

	if _, ok := us.List[id]; !ok {
		return UserNotFound
	}

	u := us.List[id]

	if displayName != nil {
		u.DisplayName = *displayName
	}
	if email != nil {
		u.Email = *email
	}

	us.List[id] = u

	err = overwriteUserStore(us)

	return
}

func dbDeleteUser(id uint) (err error) {
	us, err := getUserStore()
	if err != nil {
		return
	}

	if _, ok := us.List[id]; !ok {
		return UserNotFound
	}

	delete(us.List, id)

	err = overwriteUserStore(us)

	return
}
