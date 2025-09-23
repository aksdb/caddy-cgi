/*
 * Copyright (c) 2025 - Windows CGI Header Fix
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 */

package cgi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// WindowsCGIHandler is a custom CGI handler that fixes Windows line ending issues
type WindowsCGIHandler struct {
	cgi.Handler
}

// ServeHTTP handles the CGI request with Windows line ending fixes
func (h *WindowsCGIHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Only apply Windows fix on Windows systems
	if runtime.GOOS != "windows" {
		h.Handler.ServeHTTP(rw, req)
		return
	}

	// Execute CGI process manually to handle headers properly
	cmd := &exec.Cmd{
		Path: h.Path,
		Args: append([]string{h.Path}, h.Args...),
		Dir:  h.Dir,
		Env:  h.buildEnv(req),
	}

	if req.ContentLength != 0 {
		cmd.Stdin = req.Body
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(rw, fmt.Sprintf("CGI error: %v", err), http.StatusInternalServerError)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		http.Error(rw, fmt.Sprintf("CGI error: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(rw, fmt.Sprintf("CGI error: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse headers with Windows line ending fix
	headers, statusCode, bodyReader, err := h.parseWindowsHeaders(stdout)
	if err != nil {
		http.Error(rw, fmt.Sprintf("CGI header error: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy stderr if handler has Stderr set
	if h.Stderr != nil {
		go func() {
			io.Copy(h.Stderr, stderr)
		}()
	}

	// Set headers
	for key, values := range headers {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}

	// Set status code
	if statusCode != 0 {
		rw.WriteHeader(statusCode)
	}

	// Copy body
	io.Copy(rw, bodyReader)

	cmd.Wait()
}

// parseWindowsHeaders parses CGI headers with support for Windows \r\r\n line endings
func (h *WindowsCGIHandler) parseWindowsHeaders(reader io.Reader) (http.Header, int, io.Reader, error) {
	headers := make(http.Header)
	statusCode := 200

	// Use a buffered reader to read the response
	buf := bufio.NewReader(reader)

	// Read headers line by line, handling Windows \r\r\n issue
	for {
		line, err := h.readWindowsLine(buf)
		if err != nil {
			return nil, 0, nil, fmt.Errorf("error reading CGI headers: %v", err)
		}

		// Empty line indicates end of headers
		if len(line) == 0 {
			break
		}

		// Parse header
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue // Skip malformed headers
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Handle special Status header
		if strings.ToLower(key) == "status" {
			if len(value) >= 3 {
				if code, err := strconv.Atoi(value[:3]); err == nil {
					statusCode = code
				}
			}
		} else {
			headers.Add(key, value)
		}
	}

	return headers, statusCode, buf, nil
}

// readWindowsLine reads a line handling both \r\n and \r\r\n line endings
func (h *WindowsCGIHandler) readWindowsLine(reader *bufio.Reader) (string, error) {
	var line []byte

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				return string(line), nil
			}
			return "", err
		}

		if b == '\n' {
			// End of line found
			break
		} else if b == '\r' {
			// Check for Windows \r\r\n pattern
			peek, err := reader.Peek(2)
			if err == nil && len(peek) >= 2 && peek[0] == '\r' && peek[1] == '\n' {
				// Found \r\r\n pattern, consume the extra \r and \n
				reader.ReadByte() // consume the second \r
				reader.ReadByte() // consume the \n
				break
			} else if err == nil && len(peek) >= 1 && peek[0] == '\n' {
				// Found \r\n pattern, consume the \n
				reader.ReadByte()
				break
			}
			// Standalone \r, add to line
			line = append(line, b)
		} else {
			line = append(line, b)
		}
	}

	return string(line), nil
}

// buildEnv builds the CGI environment variables
func (h *WindowsCGIHandler) buildEnv(req *http.Request) []string {
	env := []string{
		"REQUEST_METHOD=" + req.Method,
		"SCRIPT_NAME=" + req.URL.Path,
		"PATH_INFO=" + req.URL.Path,
		"QUERY_STRING=" + req.URL.RawQuery,
		"CONTENT_TYPE=" + req.Header.Get("Content-Type"),
		"CONTENT_LENGTH=" + strconv.FormatInt(req.ContentLength, 10),
		"SERVER_NAME=" + req.Host,
		"SERVER_PORT=" + getPort(req),
		"SERVER_PROTOCOL=" + req.Proto,
		"SERVER_SOFTWARE=caddy-cgi",
		"GATEWAY_INTERFACE=CGI/1.1",
		"REMOTE_ADDR=" + req.RemoteAddr,
		"REMOTE_HOST=" + req.RemoteAddr,
	}

	// Add HTTP headers as CGI variables
	for key, values := range req.Header {
		if len(values) > 0 {
			key = strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
			env = append(env, "HTTP_"+key+"="+values[0])
		}
	}

	// Add custom environment variables
	env = append(env, h.Env...)

	// Add inherited environment variables
	if h.InheritEnv != nil {
		for _, key := range h.InheritEnv {
			if value := os.Getenv(key); value != "" {
				env = append(env, key+"="+value)
			}
		}
	}

	return env
}

// getPort extracts the port from the request
func getPort(req *http.Request) string {
	if parts := strings.Split(req.Host, ":"); len(parts) > 1 {
		return parts[1]
	}
	if req.TLS != nil {
		return "443"
	}
	return "80"
}