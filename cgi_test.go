package cgi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestCGI_ServeHTTP(t *testing.T) {
	testSetup := []struct {
		name         string
		cgi          CGI
		uri          string
		statusCode   int
		responseBody string
	}{
		{
			name: "Successful CGI request",
			cgi: CGI{
				Executable: "test/example",
				ScriptName: "/foo.cgi",
				Args:       []string{"arg1", "arg2"},
				Envs:       []string{"CGI_GLOBAL=whatever"},
				logger:     zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)),
			},
			uri:        "/foo.cgi/some/path?x=y",
			statusCode: 200,
			responseBody: `PATH_INFO [/some/path]
CGI_GLOBAL [whatever]
Arg 1 [arg1]
QUERY_STRING [x=y]
REMOTE_USER []
HTTP_TOKEN_CLAIM_USER []
CGI_LOCAL is unset`,
		},
		{
			name: "Invalid script",
			cgi: CGI{
				Executable: "test/example2",
				logger:     zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)),
			},
			uri:          "/whatever",
			statusCode:   500,
			responseBody: "",
		},
		{
			name: "Inspect",
			cgi: CGI{
				Executable: "test/example{path}",
				ScriptName: "/foo.cgi",
				Args:       []string{"arg1", "arg2"},
				Envs:       []string{"some=thing"},
				Inspect:    true,
				logger:     zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)),
			},
			uri:        "/foo.cgi/some/path?x=y",
			statusCode: 200,
			responseBody: `CGI for Caddy inspection page

Executable .................... test/example/some/path
  Arg 1 ....................... arg1
  Arg 2 ....................... arg2
Root .......................... /
Dir ........................... 
Environment
  PATH_INFO ................... /some/path
  REMOTE_USER ................. 
  SCRIPT_EXEC ................. test/example/some/path arg1 arg2
  SCRIPT_FILENAME ............. test/example/some/path
  SCRIPT_NAME ................. /foo.cgi
  some ........................ thing
Inherited environment
Placeholders
  {path} ...................... /some/path
  {root} ...................... /
  {http.request.host} ......... 
  {http.request.method} ....... 
  {http.request.uri.path} .....`,
		},
	}

	for _, testCase := range testSetup {
		t.Run(testCase.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/foo.cgi/some/path?x=y", nil)
			repl := caddy.NewReplacer()
			req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl))

			if err := testCase.cgi.ServeHTTP(res, req, NoOpNextHandler{}); err != nil {
				t.Fatalf("Cannot serve http: %v", err)
			}

			if res.Code != testCase.statusCode {
				t.Errorf("Unexpected statusCode %d. Expected %d.", res.Code, testCase.statusCode)
			}

			bodyString := strings.TrimSpace(res.Body.String())
			if bodyString != testCase.responseBody {
				t.Errorf("Unexpected body\n========== Got ==========\n%s\n========== Wanted ==========\n%s", bodyString, testCase.responseBody)
			}
		})
	}
}

func TestCGI_UnmarshalCaddyfile(t *testing.T) {
	content := `cgi /some/file a b c d 1 {
  dir /somewhere
  script_name /my.cgi
  env foo=bar what=ever
  pass_env some_env other_env
  pass_all_env
  inspect
}`
	d := caddyfile.NewTestDispenser(content)
	var c CGI
	if err := c.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("Cannot parse caddyfile: %v", err)
	}

	expected := CGI{
		Executable:       "/some/file",
		WorkingDirectory: "/somewhere",
		ScriptName:       "/my.cgi",
		Args:             []string{"a", "b", "c", "d", "1"},
		Envs:             []string{"foo=bar", "what=ever"},
		PassEnvs:         []string{"some_env", "other_env"},
		PassAll:          true,
		Inspect:          true,
	}

	if !reflect.DeepEqual(c, expected) {
		t.Fatal("Parsing yielded invalid result.")
	}
}

type NoOpNextHandler struct{}

func (n NoOpNextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	// Do Nothing
	return nil
}

func TestCGI_ServeHTTPPost(t *testing.T) {
	testSetup := []struct {
		name         string
		uri          string
		method       string
		requestBody  string
		responseBody string
		cgi          CGI
		statusCode   int
		chunked      bool
	}{
		{
			name: "POST Request",
			cgi: CGI{
				Executable: "test/example_post",
				ScriptName: "/foo.cgi",
				Args:       []string{"arg1", "arg2"},
				Envs:       []string{"CGI_GLOBAL=whatever"},
				logger:     zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)),
			},
			uri:    "foo.cgi/some/path?x=y",
			method: http.MethodPost,
			requestBody: `Chunked HTTP Request Body
With some awesome stuff in there like
this and that and also
this and that and also
this and that and also
this and that and also
this and that and also`,
			statusCode: 200,
			responseBody: `PATH_INFO [/some/path]
CGI_GLOBAL [whatever]
Arg 1 [arg1]
QUERY_STRING [x=y]
REMOTE_USER []
HTTP_TOKEN_CLAIM_USER []
CGI_LOCAL is unset
Chunked HTTP Request Body
With some awesome stuff in there like
this and that and also
this and that and also
this and that and also
this and that and also
this and that and also`,
		},
		{
			name: "POST Request with chunked Transfer-Encoding In-Memory",
			cgi: CGI{
				Executable:  "test/example_post",
				ScriptName:  "/foo.cgi",
				Args:        []string{"arg1", "arg2"},
				Envs:        []string{"CGI_GLOBAL=whatever"},
				logger:      zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)),
				BufferLimit: 200,
			},
			uri:    "foo.cgi/some/path?x=y",
			method: http.MethodPost,
			requestBody: `Chunked HTTP Request Body
With some awesome stuff in there like
this and that and also
this and that and also
this and that and also
this and that and also
this and that and also`,
			statusCode: 200,
			responseBody: `PATH_INFO [/some/path]
CGI_GLOBAL [whatever]
Arg 1 [arg1]
QUERY_STRING [x=y]
REMOTE_USER []
HTTP_TOKEN_CLAIM_USER []
CGI_LOCAL is unset
Chunked HTTP Request Body
With some awesome stuff in there like
this and that and also
this and that and also
this and that and also
this and that and also
this and that and also`,
			chunked: true,
		},
		{
			name: "POST Request with chunked Transfer-Encoding tempfile",
			cgi: CGI{
				Executable:  "test/example_post",
				ScriptName:  "/foo.cgi",
				Args:        []string{"arg1", "arg2"},
				Envs:        []string{"CGI_GLOBAL=whatever"},
				logger:      zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)),
				BufferLimit: 100,
			},
			uri:    "foo.cgi/some/path?x=y",
			method: http.MethodPost,
			requestBody: `Chunked HTTP Request Body
With some awesome stuff in there like
this and that and also
this and that and also
this and that and also
this and that and also
this and that and also`,
			statusCode: 200,
			responseBody: `PATH_INFO [/some/path]
CGI_GLOBAL [whatever]
Arg 1 [arg1]
QUERY_STRING [x=y]
REMOTE_USER []
HTTP_TOKEN_CLAIM_USER []
CGI_LOCAL is unset
Chunked HTTP Request Body
With some awesome stuff in there like
this and that and also
this and that and also
this and that and also
this and that and also
this and that and also`,
			chunked: true,
		},
	}

	for _, testCase := range testSetup {
		t.Run(testCase.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/foo.cgi/some/path?x=y", nil)

			if testCase.chunked {
				req.Header.Set("Transfer-Encoding", "chunked")
				req.TransferEncoding = []string{"chunked"}
			} else {
				cl := len(testCase.requestBody)
				req.Header.Set("Content-Length", strconv.Itoa(cl))
				req.ContentLength = int64(cl)
			}
			req.Body = io.NopCloser(strings.NewReader(testCase.requestBody))

			repl := caddy.NewReplacer()
			req = req.WithContext(context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl))

			if err := testCase.cgi.ServeHTTP(res, req, NoOpNextHandler{}); err != nil {
				t.Fatalf("Cannot serve http: %v", err)
			}

			if res.Code != testCase.statusCode {
				t.Errorf("Unexpected statusCode %d. Expected %d.", res.Code, testCase.statusCode)
			}

			bodyString := strings.TrimSpace(res.Body.String())
			if bodyString != testCase.responseBody {
				t.Errorf("Unexpected body\n========== Got ==========\n%s\n========== Wanted ==========\n%s", bodyString, testCase.responseBody)
			}
		})
	}
}
