/*
 * Copyright (c) 2018 Kurt Jung (Gmail: kurt.w.jung)
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
	"net/http"
	"net/http/cgi"
	"os"
	"sort"
	"strings"

	"github.com/caddyserver/caddy/v2"
)

type kvType struct {
	key, val string
}

func inspect(hnd cgi.Handler, w http.ResponseWriter, req *http.Request, rep *caddy.Replacer) {
	var buf bytes.Buffer

	printf := func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
	}

	kvPrint := func(indentStr, keyStr, valStr string) {
		dotLen := max(30-len(keyStr)-len(indentStr), 2)
		dotStr := strings.Repeat(".", dotLen)
		printf("%s%s %s %s\n", indentStr, keyStr, dotStr, valStr)
	}

	printf("CGI for Caddy inspection page\n\n")

	kvPrint("", "Executable", hnd.Path)

	for j, arg := range hnd.Args {
		kvPrint("  ", fmt.Sprintf("Arg %d", j+1), arg)
	}

	kvSort := func(kvList []kvType) {
		sort.Slice(kvList, func(a, b int) bool {
			return kvList[a].key < kvList[b].key
		})
	}

	split := func(list []string) (kvList []kvType) {
		for _, kv := range list {
			pair := strings.SplitN(kv, "=", 2)
			if len(pair) == 2 {
				kvList = append(kvList, kvType{key: pair[0], val: pair[1]})
			}
		}
		return
	}

	osEnv := func(list []string) (kvList []kvType) {
		for _, key := range list {
			kvList = append(kvList, kvType{key: key, val: os.Getenv(key)})
		}
		return
	}

	repPrint := func(prms ...string) {
		printf("Placeholders\n")
		for _, prm := range prms {
			kvPrint("  ", prm, rep.ReplaceAll(prm, ""))
		}
	}

	kvListPrint := func(kvList []kvType, hdrStr string) {
		printf("%s\n", hdrStr)
		kvSort(kvList)
		for _, kv := range kvList {
			kvPrint("  ", kv.key, kv.val)
		}
	}

	kvPrint("", "Root", hnd.Root)
	kvPrint("", "Dir", hnd.Dir)
	kvListPrint(split(hnd.Env), "Environment")
	kvListPrint(osEnv(hnd.InheritEnv), "Inherited environment")
	repPrint("{path}", "{root}", "{http.request.host}", "{http.request.method}", "{http.request.uri.path}")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = buf.WriteTo(w)
}
