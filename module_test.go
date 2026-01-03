package cgi

import (
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2/caddytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCGI_CaddyModule(t *testing.T) {
	tester := caddytest.NewTester(t)
	tester.InitServer(`
	{
		admin localhost:2999
		http_port     9080
		https_port    9443
	}
	localhost:9080 {
		cgi /foo* ./test/example
	}`, "caddyfile")

	resp, err := tester.Client.Get("http://localhost:9080/foo/bar")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
}
