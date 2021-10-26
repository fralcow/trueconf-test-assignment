package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type EndpointsTestSuite struct {
	suite.Suite
}

func TestMain(m *testing.M) {
	var loglevel = flag.String("loglevel", "INFO", "Set log level debug")
	flag.Parse()
	log.Debug(*loglevel)
	if loglevel != nil {
		lvl, err := log.ParseLevel(*loglevel)
		log.Debug(lvl)
		if err != nil {
			log.Error(err)
			return
		}
		log.SetLevel(lvl)
	}

	code := m.Run()

	os.Exit(code)
}

func TestEndpoints(t *testing.T) {
	suite.Run(t, new(EndpointsTestSuite))
}

func (suite *EndpointsTestSuite) SetupSuite() {
	e := os.Rename("users.json", "_users.json")
	if e != nil {
		log.Fatal(e)
	}
}

func (suite *EndpointsTestSuite) TearDownSuite() {
	e := os.Rename("_users.json", "users.json")
	if e != nil {
		log.Fatal(e)
	}
}

func (suite *EndpointsTestSuite) SetupTest() {
	f, err := os.Create("users.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	list := make(map[string]User)
	userStore := UserStore{List: list}
	data, err := json.Marshal(userStore)
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("user store data: %v", string(data))
	_, err = f.Write(data)
	if err != nil {
		log.Fatal(err)
	}
}

func (suite *EndpointsTestSuite) TearDownTest() {
	err := os.Remove("users.json")
	if err != nil {
		log.Fatal(err)
	}
}

func getUserStore() (us UserStore, err error) {

	f, err := ioutil.ReadFile("users.json")
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
	f, err := os.OpenFile("users.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	dat, err := json.Marshal(us)
	if err != nil {
		return
	}
	_, err = f.Write(dat)
	return
}

func (suite *EndpointsTestSuite) TestSearchUsers() {
	timeNow := time.Now()
	tests := []struct {
		name         string
		userStore    UserStore
		wantUserList UserList
		wantErr      error
	}{
		{
			name:         "empty user store",
			userStore:    UserStore{List: UserList{}},
			wantUserList: UserList{},
			wantErr:      nil,
		},
		{
			name: "one entry",
			userStore: UserStore{
				List: UserList{
					"1": User{
						CreatedAt:   timeNow,
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
				},
			},
			wantUserList: UserList{
				"1": User{
					CreatedAt:   timeNow,
					DisplayName: "Alice",
					Email:       "alice@email.com",
				}},
			wantErr: nil,
		},
		{
			name: "two entries",
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			//overwrite database file
			err := overwriteUserStore(test.userStore)
			if err != nil {
				log.Error(err)
				return
			}

			handler := searchUsers

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			handler(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error(err)
				return
			}
			log.Debug("GET / response:")
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			gotUserList := UserList{}
			err = json.Unmarshal(body, &gotUserList)
			if err != nil {
				log.Error(err)
				return
			}

			//we want response to match a list of users in the db
			assert.True(t,
				cmp.Equal(test.wantUserList, gotUserList),
				fmt.Sprintf("Diff: %v", cmp.Diff(test.wantUserList, gotUserList)),
			)

		})
	}
}

func (suite *EndpointsTestSuite) TestCreateUser() {
	tests := []struct {
		name           string
		wantUserStore  UserStore
		wantStatusCode int
		requestBody    string
	}{
		{
			name: "Create a user",
			wantUserStore: UserStore{
				Increment: 1,
				List: map[string]User{
					"1": {
						CreatedAt:   time.Time{},
						DisplayName: "Anne",
						Email:       "anne@email.com",
					},
				},
			},
			wantStatusCode: 201,
			requestBody:    `{"display_name": "Anne", "email": "anne@email.com"}`,
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			handler := createUser

			bodyReader := strings.NewReader(test.requestBody)
			req := httptest.NewRequest("POST", "/api/v1/users/", bodyReader)
			req.Header.Add("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error(err)
				return
			}
			log.Debug("POST /api/v1/users/ response:")
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			assert.Equal(t, test.wantStatusCode, resp.StatusCode)

			userStore, err := getUserStore()
			if err != nil {
				log.Error(err)
				return
			}

			cmpOptions := cmpopts.IgnoreFields(User{}, "CreatedAt")
			assert.True(t,
				cmp.Equal(test.wantUserStore, userStore, cmpOptions),
				fmt.Sprintf("Diff: %v", cmp.Diff(test.wantUserStore, userStore, cmpOptions)),
			)

		})
	}
}
