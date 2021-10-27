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

	userStore := UserStore{List: map[uint]User{}}
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
		name              string
		userStore         UserStore
		wantUsersResponse []UserResponse
		wantErr           error
	}{
		{
			name:              "empty user store",
			userStore:         UserStore{List: UserList{}},
			wantUsersResponse: []UserResponse{},
			wantErr:           nil,
		},
		{
			name: "one entry",
			userStore: UserStore{
				Increment: 1,
				List: UserList{
					1: User{
						CreatedAt:   timeNow,
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
				},
			},
			wantUsersResponse: []UserResponse{
				{
					User: &User{
						CreatedAt:   timeNow,
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
					Id: 1,
				},
			},
			wantErr: nil,
		},
		{
			name: "two entries",
			userStore: UserStore{
				Increment: 2,
				List: UserList{
					1: User{
						CreatedAt:   timeNow,
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
					2: User{
						CreatedAt:   timeNow,
						DisplayName: "Bob",
						Email:       "bob@email.com",
					},
				},
			},
			wantUsersResponse: []UserResponse{
				{
					User: &User{
						CreatedAt:   timeNow,
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
					Id: 1,
				},
				{
					User: &User{
						CreatedAt:   timeNow,
						DisplayName: "Bob",
						Email:       "bob@email.com",
					},
					Id: 2,
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

			gotUsersResponse := []UserResponse{}
			err = json.Unmarshal(body, &gotUsersResponse)
			if err != nil {
				t.Error(err)
				return
			}

			//we want response to match a list of users in the db
			assert.True(t,
				cmp.Equal(test.wantUsersResponse, gotUsersResponse),
				fmt.Sprintf("Diff: %v", cmp.Diff(test.wantUsersResponse, gotUsersResponse)),
			)

		})
	}
}

func (suite *EndpointsTestSuite) TestCreateUser() {

	tests := []struct {
		name             string
		requestBody      string
		wantUserStore    UserStore
		wantStatusCode   int
		wantResponseUser UserResponse
	}{
		{
			name:        "Create a user",
			requestBody: `{"display_name": "Alice", "email": "alice@email.com"}`,
			wantUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: {
						CreatedAt:   time.Time{},
						DisplayName: "Alice",
						Email:       "alice@email.com",
					},
				},
			},
			wantStatusCode: 201,
			wantResponseUser: UserResponse{
				User: &User{
					CreatedAt:   time.Time{},
					DisplayName: "Alice",
					Email:       "alice@email.com",
				},
				Id: 1,
			},
		},
		{
			name:           "Bad request",
			requestBody:    `{"disp"}`,
			wantUserStore:  UserStore{List: map[uint]User{}},
			wantStatusCode: 400,
		},
	}

	for _, test := range tests {
		overwriteUserStore(UserStore{List: map[uint]User{}})

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

			cmpOptions := cmpopts.IgnoreFields(User{}, "CreatedAt")
			if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
				gotResponse := UserResponse{}

				err = json.Unmarshal(body, &gotResponse)
				if err != nil {
					t.Error(err)
					return
				}
				assert.True(t, cmp.Equal(test.wantResponseUser, gotResponse, cmpOptions))
			}

			userStore, err := getUserStore()
			if err != nil {
				t.Error(err)
				return
			}

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
		requestUserId    int
		wantUser         User
		wantStatusCode   int
	}{
		{
			name: "Existing user",
			fixtureUserStore: UserStore{
				Increment: 1,
				List:      map[uint]User{1: userAlice},
			},
			requestUserId:  1,
			wantUser:       userAlice,
			wantStatusCode: 200,
		},
		{
			name: "Non existent user",
			fixtureUserStore: UserStore{
				Increment: 1,
				List:      map[uint]User{1: userAlice},
			},
			requestUserId:  2,
			wantUser:       User{},
			wantStatusCode: 404,
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

			req, err := http.NewRequest("GET",
				ts.URL+"/api/v1/users/"+fmt.Sprint(test.requestUserId),
				nil)
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

func (suite *EndpointsTestSuite) TestUpdateUser() {
	userAlice := User{CreatedAt: time.Now(),
		DisplayName: "Alice",
		Email:       "alice@email.com",
	}

	tests := []struct {
		name             string
		fixtureUserStore UserStore
		wantUserStore    UserStore
		requestUserId    int
		requestBody      string
		wantStatusCode   int
	}{
		{
			name: "Update user's name and email",
			fixtureUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			wantUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: {
						DisplayName: "Alice1",
						Email:       "alice1@email.com",
					},
				},
			},
			requestUserId: 1,
			requestBody: `{"display_name": "Alice1",
						   "email": "alice1@email.com"}`,
			wantStatusCode: 200,
		},
		{
			name: "Update user's name",
			fixtureUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			wantUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: {
						DisplayName: "Alice1",
						Email:       "alice@email.com",
					},
				},
			},
			requestUserId:  1,
			requestBody:    `{"display_name": "Alice1"}`,
			wantStatusCode: 200,
		},
		{
			name: "Update non existent user",
			fixtureUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			wantUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			requestUserId:  2,
			requestBody:    `{"display_name": "Alice1"}`,
			wantStatusCode: 404,
		},
		{
			name: "Bad request",
			fixtureUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			wantUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			requestUserId:  1,
			requestBody:    `{"display"}`,
			wantStatusCode: 400,
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

			bodyReader := strings.NewReader(test.requestBody)

			req, err := http.NewRequest("PATCH",
				ts.URL+"/api/v1/users/"+fmt.Sprint(test.requestUserId)+"/",
				bodyReader)
			req.Header.Add("Content-Type", "application/json")

			resp, body := testRequest(t, ts, req)

			log.Debug(req.Method, req.URL)
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			assert.Equal(t, test.wantStatusCode, resp.StatusCode)

			gotUserStore, err := getUserStore()
			if err != nil {
				t.Error(err)
				return
			}

			cmpOptions := cmpopts.IgnoreFields(User{}, "CreatedAt")
			assert.True(t, cmp.Equal(test.wantUserStore, gotUserStore, cmpOptions), cmp.Diff(test.wantUserStore, gotUserStore, cmpOptions))

		})
	}
}

func (suite *EndpointsTestSuite) TestDeleteUser() {
	userAlice := User{CreatedAt: time.Now(),
		DisplayName: "Alice",
		Email:       "alice@email.com",
	}

	tests := []struct {
		name             string
		fixtureUserStore UserStore
		wantUserStore    UserStore
		requestUserId    int
		wantStatusCode   int
	}{
		{
			name: "Delete user",
			fixtureUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			wantUserStore: UserStore{
				Increment: 1,
				List:      map[uint]User{},
			},
			requestUserId:  1,
			wantStatusCode: 200,
		},
		{
			name: "Delete non existent user",
			fixtureUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			wantUserStore: UserStore{
				Increment: 1,
				List: map[uint]User{
					1: userAlice,
				},
			},
			requestUserId:  2,
			wantStatusCode: 404,
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

			req, err := http.NewRequest("DELETE",
				ts.URL+"/api/v1/users/"+fmt.Sprint(test.requestUserId)+"/",
				nil)

			resp, body := testRequest(t, ts, req)

			log.Debug(req.Method, req.URL)
			log.Debug(resp.StatusCode)
			log.Debug(resp.Header.Get("Content-Type"))
			log.Debug(string(body))

			assert.Equal(t, test.wantStatusCode, resp.StatusCode)

			gotUserStore, err := getUserStore()
			if err != nil {
				t.Error(err)
				return
			}

			cmpOptions := cmpopts.IgnoreFields(User{}, "CreatedAt")
			assert.True(t, cmp.Equal(test.wantUserStore, gotUserStore, cmpOptions), cmp.Diff(test.wantUserStore, gotUserStore, cmpOptions))
		})
	}
}
