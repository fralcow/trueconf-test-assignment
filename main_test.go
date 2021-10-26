package main

import (
	"flag"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"

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

	dat, err := os.ReadFile("_users.json")
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.Write(dat)
	if err != nil {
		log.Fatal(err)
	}

	err = f.Sync()
	if err != nil {
		log.Fatal(err)
	}
}

func (suite *EndpointsTestSuite) TearDownTest() {
	e := os.Remove("users.json")
	if e != nil {
		log.Fatal(e)
	}
}

func (suite *EndpointsTestSuite) TestSearchUsers() {
	handler := searchUsers

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	log.Debug("GET / response:")
	log.Debug(resp.StatusCode)
	log.Debug(resp.Header.Get("Content-Type"))
	log.Debug(string(body))
}
