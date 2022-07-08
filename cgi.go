/*
 * Copyright (c) 2017 Kurt Jung (Gmail: kurt.w.jung)
 * Copyright (c) 2020 Andreas Schneider
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package cgi

import (
	"fmt"
	"net/http"
	"net/http/cgi"
	"os"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// passAll returns a slice of strings made up of each environment key
func passAll() (list []string) {
	envList := os.Environ() // ["HOME=/home/foo", "LVL=2", ...]
	for _, str := range envList {
		pos := strings.Index(str, "=")
		if pos > 0 {
			list = append(list, str[:pos])
		}
	}
	return
}

func (c *CGI) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// For convenience: get the currently authenticated user; if some other middleware has set that.
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var username string
	if usernameVal, exists := repl.Get("http.auth.user.id"); exists {
		if usernameVal, ok := usernameVal.(string); ok {
			username = usernameVal
		}
	}

	scriptPath := strings.TrimPrefix(r.URL.Path, c.ScriptName)

	var cgiHandler cgi.Handler

	cgiHandler.Root = "/"

	repl.Set("root", cgiHandler.Root)
	repl.Set("path", scriptPath)

	cgiHandler.Dir = c.WorkingDirectory
	cgiHandler.Path = repl.ReplaceAll(c.Executable, "")
	for _, str := range c.Args {
		cgiHandler.Args = append(cgiHandler.Args, repl.ReplaceAll(str, ""))
	}

	envAdd := func(key, val string) {
		val = repl.ReplaceAll(val, "")
		cgiHandler.Env = append(cgiHandler.Env, key+"="+val)
	}
	envAdd("PATH_INFO", scriptPath)
	envAdd("SCRIPT_FILENAME", cgiHandler.Path)
	envAdd("SCRIPT_NAME", c.ScriptName)
	envAdd("SCRIPT_EXEC", fmt.Sprintf("%s %s", cgiHandler.Path, strings.Join(cgiHandler.Args, " ")))
	envAdd("REMOTE_USER", username)

	for _, e := range c.Envs {
		cgiHandler.Env = append(cgiHandler.Env, repl.ReplaceAll(e, ""))
	}

	if c.PassAll {
		cgiHandler.InheritEnv = passAll()
	} else {
		cgiHandler.InheritEnv = append(cgiHandler.InheritEnv, c.PassEnvs...)
	}

	if c.Inspect {
		inspect(cgiHandler, w, r, repl)
	} else {
		cgiHandler.ServeHTTP(w, r)
	}
	return next.ServeHTTP(w, r)
}
