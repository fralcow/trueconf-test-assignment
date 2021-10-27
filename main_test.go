package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var r *chi.Mux

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

	r = chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	//r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	setRoutes(r)
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

	userStore := UserStore{List: map[string]User{}}
	err = overwriteUserStore(userStore)
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
	log.Debugf("UserStore data: %v", string(dat))
	return
}

func testRequest(t *testing.T, ts *httptest.Server, req *http.Request) (response *http.Response, responseBody []byte) {
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return
	}

	responseBody, err = ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer response.Body.Close()

	return
}

func (suite *EndpointsTestSuite) TestServer() {
	go main()

	// give the server some time to start
	time.Sleep(1 * time.Second)

	resp, err := http.Get("http://127.0.0.1:3333/")
	suite.NoError(err)

	suite.Equal(200, resp.StatusCode)

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
				Increment: 1,
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
			userStore: UserStore{
				Increment: 2,
				List: UserList{
					"1": User{
						CreatedAt:   timeNow,
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
					"2": User{
						CreatedAt:   timeNow,
						DisplayName: "Bob",
						Email:       "bob@email.com",
					},
				},
			},
			wantUserList: UserList{
				"1": User{
					CreatedAt:   timeNow,
					DisplayName: "Alice",
					Email:       "alice@email.com",
				},
				"2": User{
					CreatedAt:   timeNow,
					DisplayName: "Bob",
					Email:       "bob@email.com",
				},
			},
			wantErr: nil,
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {
			//overwrite database file
			err := overwriteUserStore(test.userStore)
			if err != nil {
				t.Error(err)
				return
			}

			ts := httptest.NewServer(r)
			defer ts.Close()

			req, err := http.NewRequest("GET", ts.URL+"/api/v1/users/", nil)
			if err != nil {
				t.Error(err)
				return
			}
			req.Header.Add("Accept", "application/json")

			resp, body := testRequest(t, ts, req)

			log.Debug(req.Method, req.URL)
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			gotUserList := UserList{}
			err = json.Unmarshal(body, &gotUserList)
			if err != nil {
				t.Error(err)
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
		requestBody    string
		wantStatusCode int
	}{
		{
			name: "Create a user",
			wantUserStore: UserStore{
				Increment: 1,
				List: map[string]User{
					"1": {
						CreatedAt:   time.Time{},
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
				},
			},
			wantStatusCode: 201,
			requestBody:    `{"display_name": "Alice", "email": "alice@email.com"}`,
		},
		{
			name:           "Bad request",
			wantUserStore:  UserStore{List: map[string]User{}},
			wantStatusCode: 400,
			requestBody:    `{"disp"}`,
		},
	}

	for _, test := range tests {
		overwriteUserStore(UserStore{List: map[string]User{}})

		suite.T().Run(test.name, func(t *testing.T) {

			bodyReader := strings.NewReader(test.requestBody)

			ts := httptest.NewServer(r)
			defer ts.Close()

			req, err := http.NewRequest("POST", ts.URL+"/api/v1/users", bodyReader)
			if err != nil {
				t.Error(err)
				return
			}
			req.Header.Add("Content-Type", "application/json")

			resp, body := testRequest(t, ts, req)

			log.Debug(req.Method, req.URL)
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			assert.Equal(t, test.wantStatusCode, resp.StatusCode)

			userStore, err := getUserStore()
			if err != nil {
				t.Error(err)
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

func (suite *EndpointsTestSuite) TestGetUser() {
	userAlice := User{CreatedAt: time.Now(),
		DisplayName: "Alice",
		Email:       "alice@email.com",
	}
	tests := []struct {
		name             string
		fixtureUserStore UserStore
		wantStatusCode   int
		wantUser         User
	}{
		{
			name: "Existing user",
			fixtureUserStore: UserStore{
				Increment: 1,
				List:      map[string]User{"1": userAlice},
			},
			wantStatusCode: 200,
			wantUser:       userAlice,
		},
	}

	for _, test := range tests {
		suite.T().Run(test.name, func(t *testing.T) {

			//overwrite database file
			err := overwriteUserStore(test.fixtureUserStore)
			if err != nil {
				t.Error(err)
				return
			}

			ts := httptest.NewServer(r)
			defer ts.Close()

			req, err := http.NewRequest("GET", ts.URL+"/api/v1/users/1", nil)
			resp, body := testRequest(t, ts, req)

			log.Debug(req.Method, req.URL)
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			assert.Equal(t, test.wantStatusCode, resp.StatusCode)

			gotUser := User{}
			err = json.Unmarshal(body, &gotUser)
			if err != nil {
				t.Error(err)
				return
			}

			assert.True(t, cmp.Equal(test.wantUser, gotUser), cmp.Diff(test.wantUser, gotUser))

		})
	}
}
