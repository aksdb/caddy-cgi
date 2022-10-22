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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cgi"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

var bufPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

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

	scriptName := repl.ReplaceAll(c.ScriptName, "")
	scriptPath := strings.TrimPrefix(r.URL.Path, scriptName)

	var cgiHandler cgi.Handler

	cgiHandler.Root = "/"

	repl.Set("root", cgiHandler.Root)
	repl.Set("path", scriptPath)

	errorBuffer := bufPool.Get().(*bytes.Buffer)
	errorBuffer.Reset()
	defer bufPool.Put(errorBuffer)

	cgiHandler.Dir = c.WorkingDirectory
	cgiHandler.Path = repl.ReplaceAll(c.Executable, "")
	cgiHandler.Stderr = errorBuffer
	for _, str := range c.Args {
		cgiHandler.Args = append(cgiHandler.Args, repl.ReplaceAll(str, ""))
	}

	envAdd := func(key, val string) {
		cgiHandler.Env = append(cgiHandler.Env, key+"="+val)
	}
	envAdd("PATH_INFO", scriptPath)
	envAdd("SCRIPT_FILENAME", cgiHandler.Path)
	envAdd("SCRIPT_NAME", scriptName)
	envAdd("SCRIPT_EXEC", fmt.Sprintf("%s %s", cgiHandler.Path, strings.Join(cgiHandler.Args, " ")))
	envAdd("REMOTE_USER", username)

	// work around Go's CGI not handling chunked transfer encodings
	// https://github.com/golang/go/issues/5613
	if len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked" {
		// buffer request in memory or temporary file if too large
		// to make it possible to calculate the CONTENT_LENGTH of the body
		defer r.Body.Close()

		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufPool.Put(buf)
		if buf.Cap() < int(c.BufferLimit) {
			buf.Grow(int(c.BufferLimit) + bytes.MinRead)
		}

		size, err := io.CopyN(buf, r.Body, c.BufferLimit)
		if err != nil && err != io.EOF {
			return err
		}

		// if the buffer is full there is probably more,
		// so use a tempfile to read the rest and use that as request body
		if size == c.BufferLimit {
			tempfile, err := os.CreateTemp("", "cgi_body_*")
			if err != nil {
				return err
			}
			defer os.Remove(tempfile.Name())
			defer tempfile.Close()

			// write the already read bytes
			_, err = tempfile.Write(buf.Bytes())
			if err != nil {
				return err
			}

			// reuse the bytes slice of the buffer to copy the rest of the body to the tempfile
			remainingSize, err := io.CopyBuffer(tempfile, r.Body, buf.Bytes())
			if err != nil {
				return err
			}
			size += remainingSize

			// seek to start, so it can be read from the beginning
			_, err = tempfile.Seek(0, io.SeekStart)
			if err != nil {
				return err
			}
			r.Body = tempfile
		} else {
			r.Body = io.NopCloser(buf)
		}

		// all the request body is read, so it isn't chunked anymore
		r.TransferEncoding = nil
		r.Header.Del("Transfer-Encoding")

		// we can set the size of the request body now that we read everything
		sizeStr := strconv.FormatInt(size, 10)
		r.Header.Add("Content-Length", sizeStr)
		r.ContentLength = size
	}

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
		cgiWriter := w
		if c.UnbufferedOutput {
			cgiWriter = instantWriter{w}
		}
		cgiHandler.ServeHTTP(cgiWriter, r)
	}

	if c.logger != nil && errorBuffer.Len() > 0 {
		c.logger.Error("Error from CGI Application", zap.Stringer("Stderr", errorBuffer))
	}

	return nil
}

type instantWriter struct {
	http.ResponseWriter
}

func (iw instantWriter) Write(b []byte) (int, error) {
	n, err := iw.ResponseWriter.Write(b)
	iw.ResponseWriter.(http.Flusher).Flush()
	return n, err
}
