package cgi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
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
			},
			uri:          "/whatever",
			statusCode:   500,
			responseBody: "",
		},
		{
			name: "Inspect",
			cgi: CGI{
				Executable: "test/example",
				ScriptName: "/foo.cgi",
				Args:       []string{"arg1", "arg2"},
				Envs:       []string{"some=thing"},
				Inspect:    true,
			},
			uri:        "/foo.cgi/some/path?x=y",
			statusCode: 200,
			responseBody: `CGI for Caddy inspection page

Executable .................... test/example
  Arg 1 ....................... arg1
  Arg 2 ....................... arg2
Root .......................... /
Dir ........................... 
Environment
  PATH_INFO ................... /some/path
  REMOTE_USER ................. 
  SCRIPT_FILENAME ............. test/example
  SCRIPT_NAME ................. /foo.cgi
  some ........................ thing
Inherited environment`,
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
