// Copyright 2014 Canonical Ltd.

package identity_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/juju/httprequest"
	"github.com/juju/idmclient/params"
	"github.com/juju/loggo"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/testing/httptesting"
	"github.com/julienschmidt/httprouter"
	gc "gopkg.in/check.v1"

	"github.com/CanonicalLtd/blues-identity/internal/identity"
	"github.com/CanonicalLtd/blues-identity/internal/store"
)

type serverSuite struct {
	testing.IsolatedMgoSuite
}

var _ = gc.Suite(&serverSuite{})

func (s *serverSuite) TestNewServerWithNoVersions(c *gc.C) {
	h, err := identity.New(s.Session.DB("foo"), identity.ServerParams{
		PrivateAddr: "localhost",
	}, nil)
	c.Assert(err, gc.ErrorMatches, `identity server must serve at least one version of the API`)
	c.Assert(h, gc.IsNil)
}

type versionResponse struct {
	Version string
	Path    string
}

func (s *serverSuite) TestNewServerWithVersions(c *gc.C) {
	db := s.Session.DB("foo")
	serveVersion := func(vers string) identity.NewAPIHandlerFunc {
		return func(*store.Pool, identity.ServerParams) ([]httprequest.Handler, error) {
			return []httprequest.Handler{{
				Method: "GET",
				Path:   "/" + vers + "/*path",
				Handle: func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
					w.Header().Set("Content-Type", "application/json")
					response := versionResponse{
						Version: vers,
						Path:    req.URL.Path,
					}
					enc := json.NewEncoder(w)
					err := enc.Encode(response)
					c.Assert(err, gc.IsNil)
				},
			}}, nil
		}
	}

	h, err := identity.New(db, identity.ServerParams{
		PrivateAddr: "localhost",
	}, map[string]identity.NewAPIHandlerFunc{
		"version1": serveVersion("version1"),
	})
	c.Assert(err, gc.IsNil)
	defer h.Close()
	assertServesVersion(c, h, "version1")
	assertDoesNotServeVersion(c, h, "version2")
	assertDoesNotServeVersion(c, h, "version3")

	h, err = identity.New(db, identity.ServerParams{
		PrivateAddr: "localhost",
	}, map[string]identity.NewAPIHandlerFunc{
		"version1": serveVersion("version1"),
		"version2": serveVersion("version2"),
	})
	c.Assert(err, gc.IsNil)
	defer h.Close()
	assertServesVersion(c, h, "version1")
	assertServesVersion(c, h, "version2")
	assertDoesNotServeVersion(c, h, "version3")

	h, err = identity.New(db, identity.ServerParams{
		PrivateAddr: "localhost",
	}, map[string]identity.NewAPIHandlerFunc{
		"version1": serveVersion("version1"),
		"version2": serveVersion("version2"),
		"version3": serveVersion("version3"),
	})
	c.Assert(err, gc.IsNil)
	defer h.Close()
	assertServesVersion(c, h, "version1")
	assertServesVersion(c, h, "version2")
	assertServesVersion(c, h, "version3")
}

func (s *serverSuite) TestServerHasAccessControlAllowHeaders(c *gc.C) {
	db := s.Session.DB("foo")
	impl := map[string]identity.NewAPIHandlerFunc{
		"/a": func(*store.Pool, identity.ServerParams) ([]httprequest.Handler, error) {
			return []httprequest.Handler{{
				Method: "GET",
				Path:   "/a",
				Handle: func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
				},
			}}, nil
		},
	}

	h, err := identity.New(db, identity.ServerParams{
		PrivateAddr: "localhost",
	}, impl)
	c.Assert(err, gc.IsNil)
	defer h.Close()
	rec := httptesting.DoRequest(c, httptesting.DoRequestParams{
		Handler: h,
		URL:     "/a",
	})
	c.Assert(rec.Code, gc.Equals, http.StatusOK)
	c.Assert(len(rec.HeaderMap["Access-Control-Allow-Origin"]), gc.Equals, 1)
	c.Assert(rec.HeaderMap["Access-Control-Allow-Origin"][0], gc.Equals, "*")
	c.Assert(len(rec.HeaderMap["Access-Control-Allow-Headers"]), gc.Equals, 1)
	c.Assert(rec.HeaderMap["Access-Control-Allow-Headers"][0], gc.Equals, "Bakery-Protocol-Version, Macaroons, X-Requested-With, Content-Type")
	c.Assert(len(rec.HeaderMap["Access-Control-Allow-Origin"]), gc.Equals, 1)
	c.Assert(rec.HeaderMap["Access-Control-Allow-Origin"][0], gc.Equals, "*")
	c.Assert(len(rec.HeaderMap["Access-Control-Cache-Max-Age"]), gc.Equals, 1)
	c.Assert(rec.HeaderMap["Access-Control-Cache-Max-Age"][0], gc.Equals, "600")

	rec = httptesting.DoRequest(c, httptesting.DoRequestParams{
		Handler: h,
		URL:     "/a/",
		Method:  "OPTIONS",
		Header:  http.Header{"Origin": []string{"MyHost"}},
	})
	c.Assert(rec.Code, gc.Equals, http.StatusOK)
	c.Assert(len(rec.HeaderMap["Access-Control-Allow-Origin"]), gc.Equals, 1)
	c.Assert(rec.HeaderMap["Access-Control-Allow-Origin"][0], gc.Equals, "*")
}

func (s *serverSuite) TestServerPanicRecovery(c *gc.C) {
	w := new(loggo.TestWriter)
	loggo.RegisterWriter("test", w)
	db := s.Session.DB("foo")
	impl := map[string]identity.NewAPIHandlerFunc{
		"/a": func(*store.Pool, identity.ServerParams) ([]httprequest.Handler, error) {
			return []httprequest.Handler{{
				Method: "GET",
				Path:   "/a",
				Handle: func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
					panic("test panic")
				},
			}}, nil
		},
	}

	h, err := identity.New(db, identity.ServerParams{
		PrivateAddr: "localhost",
	}, impl)
	c.Assert(err, gc.IsNil)
	defer h.Close()
	httptesting.AssertJSONCall(c, httptesting.JSONCallParams{
		Handler:      h,
		URL:          "/a",
		ExpectStatus: http.StatusInternalServerError,
		ExpectBody: params.Error{
			Code:    "panic",
			Message: "test panic",
		},
	})
	c.Assert(w.Log(), jc.LogMatches, []jc.SimpleMessage{{loggo.ERROR, `PANIC!: test panic\n.*`}})
}

func (s *serverSuite) TestServerStaticFiles(c *gc.C) {
	db := s.Session.DB("foo")
	serveVersion := func(vers string) identity.NewAPIHandlerFunc {
		return func(*store.Pool, identity.ServerParams) ([]httprequest.Handler, error) {
			return []httprequest.Handler{{
				Method: "GET",
				Path:   "/" + vers + "/*path",
				Handle: func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
					w.Header().Set("Content-Type", "application/json")
					response := versionResponse{
						Version: vers,
						Path:    req.URL.Path,
					}
					enc := json.NewEncoder(w)
					err := enc.Encode(response)
					c.Assert(err, gc.IsNil)
				},
			}}, nil
		}
	}
	path := c.MkDir()
	h, err := identity.New(db, identity.ServerParams{
		PrivateAddr:      "localhost",
		StaticFileSystem: http.Dir(path),
	}, map[string]identity.NewAPIHandlerFunc{
		"version1": serveVersion("version1"),
	})
	c.Assert(err, gc.IsNil)
	defer h.Close()

	f, err := os.Create(filepath.Join(path, "file"))
	c.Assert(err, gc.IsNil)
	fmt.Fprintf(f, "test file")
	f.Close()

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/static/file", nil)
	c.Assert(err, gc.IsNil)
	h.ServeHTTP(rr, req)
	c.Assert(rr.Code, gc.Equals, http.StatusOK, gc.Commentf("%d: %s", rr.Code, rr.Body.String()))
	c.Assert(rr.Body.String(), gc.Equals, "test file")
}

func assertServesVersion(c *gc.C, h http.Handler, vers string) {
	path := vers
	if path != "" {
		path = "/" + path
	}
	httptesting.AssertJSONCall(c, httptesting.JSONCallParams{
		Handler: h,
		URL:     path + "/some/path",
		ExpectBody: versionResponse{
			Version: vers,
			Path:    "/" + vers + "/some/path",
		},
	})
}

func assertDoesNotServeVersion(c *gc.C, h http.Handler, vers string) {
	rec := httptesting.DoRequest(c, httptesting.DoRequestParams{
		Handler: h,
		URL:     "/" + vers + "/some/path",
	})
	c.Assert(rec.Code, gc.Equals, http.StatusNotFound)
}
