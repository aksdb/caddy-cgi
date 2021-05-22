# CGI for Caddy

[![MIT
licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/aksdb/caddy-cgi/master/LICENSE)
[![Report](https://goreportcard.com/badge/github.com/aksdb/caddy-cgi)](https://goreportcard.com/report/github.com/aksdb/caddy-cgi)

Package cgi implements the common gateway interface
([CGI](https://en.wikipedia.org/wiki/Common_Gateway_Interface)) for
[Caddy 2](https://caddyserver.com/), a modern, full-featured,
easy-to-use web server.

It has been forked from the fantastic work of [Kurt
Jung](https://github.com/jung-kurt/caddy-cgi) who wrote that plugin for
Caddy 1.

## Documentation

This plugin lets you generate dynamic content on your website by means
of command line scripts. To collect information about the inbound HTTP
request, your script examines certain environment variables such as
`PATH_INFO` and `QUERY_STRING`. Then, to return a dynamically generated
web page to the client, your script simply writes content to standard
output. In the case of POST requests, your script reads additional
inbound content from standard input.

The advantage of CGI is that you do not need to fuss with server startup
and persistence, long term memory management, sockets, and crash
recovery. Your script is called when a request matches one of the
patterns that you specify in your Caddyfile. As soon as your script
completes its response, it terminates. This simplicity makes CGI a
perfect complement to the straightforward operation and configuration of
Caddy. The benefits of Caddy, including HTTPS by default, basic access
authentication, and lots of middleware options extend easily to your CGI
scripts.

CGI has some disadvantages. For one, Caddy needs to start a new process
for each request. This can adversely impact performance and, if
resources are shared between CGI applications, may require the use of
some interprocess synchronization mechanism such as a file lock. Your
server’s responsiveness could in some circumstances be affected, such as
when your web server is hit with very high demand, when your script’s
dependencies require a long startup, or when concurrently running
scripts take a long time to respond. However, in many cases, such as
using a pre-compiled CGI application like fossil or a Lua script, the
impact will generally be insignificant. Another restriction of CGI is
that scripts will be run with the same permissions as Caddy itself. This
can sometimes be less than ideal, for example when your script needs to
read or write files associated with a different owner.

### Security Considerations

Serving dynamic content exposes your server to more potential threats
than serving static pages. There are a number of considerations of which
you should be aware when using CGI applications.

<div class="warning">

**CGI scripts should be located outside of Caddy’s document root.**
Otherwise, an inadvertent misconfiguration could result in Caddy
delivering the script as an ordinary static resource. At best, this
could merely confuse the site visitor. At worst, it could expose
sensitive internal information that should not leave the server.

</div>

<div class="warning">

**Mistrust the contents of `PATH_INFO`, `QUERY_STRING` and standard
input.** Most of the environment variables available to your CGI program
are inherently safe because they originate with Caddy and cannot be
modified by external users. This is not the case with `PATH_INFO`,
`QUERY_STRING` and, in the case of POST actions, the contents of
standard input. Be sure to validate and sanitize all inbound content. If
you use a CGI library or framework to process your scripts, make sure
you understand its limitations.

</div>

### Errors

An error in a CGI application is generally handled within the
application itself and reported in the headers it returns.

### Application Modes

Your CGI application can be executed directly or indirectly. In the
direct case, the application can be a compiled native executable or it
can be a shell script that contains as its first line a shebang that
identifies the interpreter to which the file’s name should be passed.
Caddy must have permission to execute the application. On Posix systems
this will mean making sure the application’s ownership and permission
bits are set appropriately; on Windows, this may involve properly
setting up the filename extension association.

In the indirect case, the name of the CGI script is passed to an
interpreter such as lua, perl or python.

### Requirements

  - This module needs to be installed (obviously).
    
    Refer to the Caddy documentation on how to build Caddy with
    plugins/modules.

  - The directive needs to be registered in the Caddyfile:
    
    ``` caddy
    {
        order cgi last
    }
    ```

### Basic Syntax

The basic cgi directive lets you add a handler in the current caddy
router location with a given script and optional arguments. The matcher
is a default caddy matcher that is used to restrict the scope of this
directive. The directive can be repeated any reasonable number of times.
Here is the basic syntax:

``` caddy
cgi [matcher] exec [args...]
```

For example:

``` caddy
cgi /report /usr/local/cgi-bin/report
```

When a request such as https://example.com/report or
https://example.com/report/weekly arrives, the cgi middleware will
detect the match and invoke the script named /usr/local/cgi-bin/report.
The current working directory will be the same as Caddy itself. Here, it
is assumed that the script is self-contained, for example a pre-compiled
CGI application or a shell script. Here is an example of a standalone
script, similar to one used in the cgi plugin’s test suite:

``` shell
#!/bin/bash

printf "Content-type: text/plain\n\n"

printf "PATH_INFO    [%s]\n" $PATH_INFO
printf "QUERY_STRING [%s]\n" $QUERY_STRING

exit 0
```

The environment variables `PATH_INFO` and `QUERY_STRING` are populated
and passed to the script automatically. There are a number of other
standard CGI variables included that are described below. If you need to
pass any special environment variables or allow any environment
variables that are part of Caddy’s process to pass to your script, you
will need to use the advanced directive syntax described below.

Beware that in Caddy v2 it is (currently) not possible to separate the
path left of the matcher from the full URL. Therefore if you require
your CGI program to know the `SCRIPT_NAME`, make sure to pass that
explicitly:

``` caddy
cgi /script.cgi* /path/to/my/script someargument {
  script_name /script.cgi
}
```

### Advanced Syntax

In order to specify custom environment variables, pass along one or more
environment variables known to Caddy, or specify more than one match
pattern for a given rule, you will need to use the advanced directive
syntax. That looks like this:

``` caddy
cgi [matcher] exec [args...] {
    script_name subpath
    dir working_directory
    env key1=val1 [key2=val2...]
    pass_env key1 [key2...]
    pass_all_env
    inspect
}
```

For example,

``` caddy
cgi /sample/report* /usr/local/bin/reportscript.sh {
    script_name /sample/report
    env DB=/usr/local/share/app/app.db SECRET=/usr/local/share/app/secret CGI_LOCAL=
    pass_env HOME UID
}
```

The `script_name` subdirective helps the cgi module to separate the path
to the script from the (virtual) path afterwards (which shall be passed
to the script).

`env` can be used to define a list of `key=value` environment variable
pairs that shall be passed to the script. `pass_env` can be used to
define a list of environment variables of the Caddy process that shall
be passed to the script.

If your CGI application runs properly at the command line but fails to
run from Caddy it is possible that certain environment variables may be
missing. For example, the ruby gem loader evidently requires the `HOME`
environment variable to be set; you can do this with the subdirective
`pass_env HOME`. Another class of problematic applications require the
`COMPUTERNAME` variable.

The `pass_all_env` subdirective instructs Caddy to pass each environment
variable it knows about to the CGI excutable. This addresses a common
frustration that is caused when an executable requires an environment
variable and fails without a descriptive error message when the variable
cannot be found. These applications often run fine from the command
prompt but fail when invoked with CGI. The risk with this subdirective
is that a lot of server information is shared with the CGI executable.
Use this subdirective only with CGI applications that you trust not to
leak this information.

### Troubleshooting

If you run into unexpected results with the CGI plugin, you are able to
examine the environment in which your CGI application runs. To enter
inspection mode, add the subdirective `inspect` to your CGI
configuration block. This is a development option that should not be
used in production. When in inspection mode, the plugin will respond to
matching requests with a page that displays variables of interest. In
particular, it will show the replacement value of `{match}` and the
environment variables to which your CGI application has access.

For example, consider this example CGI block:

``` caddy
cgi /wapp/*.tcl /usr/local/bin/wapptclsh /home/quixote/projects{path} {
    script_name /wapp
    pass_env HOME LANG
    env DB=/usr/local/share/app/app.db SECRET=/usr/local/share/app/secret
    inspect
}
```

When you request a matching URL, for example,

    https://example.com/wapp/hello.tcl

the Caddy server will deliver a text page similar to the following. The
CGI application (in this case, wapptclsh) will not be called.

    CGI for Caddy inspection page
    
    Executable .................... /usr/local/bin/wapptclsh
      Arg 1 ....................... /home/quixote/projects/hello.tcl
    Root .......................... /
    Dir ........................... /home/quixote/www
    Environment
      DB .......................... /usr/local/share/app/app.db
      PATH_INFO ...................
      REMOTE_USER .................
      SCRIPT_EXEC ................. /usr/local/bin/wapptclsh /home/quixote/projects/hello.tcl
      SCRIPT_FILENAME ............. /usr/local/bin/wapptclsh
      SCRIPT_NAME ................. /wapp/hello
      SECRET ...................... /usr/local/share/app/secret
    Inherited environment
      HOME ........................ /home/quixote
      LANG ........................ en_US.UTF-8
    Placeholders
      {path} ...................... /hello
      {root} ...................... /
      {http.request.host} ......... example.com
      {http.request.host} ......... GET
      {http.request.host} ......... /wapp/hello.tcl

This information can be used to diagnose problems with how a CGI
application is called.

To return to operation mode, remove or comment out the `inspect`
subdirective.

### Environment Variable Example

In this example, the Caddyfile looks like this:

``` caddy
{
    http_port 8080
    order cgi last
}

192.168.1.2:8080
cgi /show* /usr/local/cgi-bin/report/gen {
    script_name /show
}
```

Note that a request for /show gets mapped to a script named
/usr/local/cgi-bin/report/gen. There is no need for any element of the
script name to match any element of the match pattern.

The contents of /usr/local/cgi-bin/report/gen are:

``` shell
#!/bin/bash

printf "Content-type: text/plain\n\n"

printf "example error message\n" > /dev/stderr

if [ "POST" = "$REQUEST_METHOD" -a -n "$CONTENT_LENGTH" ]; then
  read -n "$CONTENT_LENGTH" POST_DATA
fi

printf "AUTH_TYPE         [%s]\n" $AUTH_TYPE
printf "CONTENT_LENGTH    [%s]\n" $CONTENT_LENGTH
printf "CONTENT_TYPE      [%s]\n" $CONTENT_TYPE
printf "GATEWAY_INTERFACE [%s]\n" $GATEWAY_INTERFACE
printf "PATH_INFO         [%s]\n" $PATH_INFO
printf "PATH_TRANSLATED   [%s]\n" $PATH_TRANSLATED
printf "POST_DATA         [%s]\n" $POST_DATA
printf "QUERY_STRING      [%s]\n" $QUERY_STRING
printf "REMOTE_ADDR       [%s]\n" $REMOTE_ADDR
printf "REMOTE_HOST       [%s]\n" $REMOTE_HOST
printf "REMOTE_IDENT      [%s]\n" $REMOTE_IDENT
printf "REMOTE_USER       [%s]\n" $REMOTE_USER
printf "REQUEST_METHOD    [%s]\n" $REQUEST_METHOD
printf "SCRIPT_EXEC       [%s]\n" $SCRIPT_EXEC
printf "SCRIPT_NAME       [%s]\n" $SCRIPT_NAME
printf "SERVER_NAME       [%s]\n" $SERVER_NAME
printf "SERVER_PORT       [%s]\n" $SERVER_PORT
printf "SERVER_PROTOCOL   [%s]\n" $SERVER_PROTOCOL
printf "SERVER_SOFTWARE   [%s]\n" $SERVER_SOFTWARE

exit 0
```

The purpose of this script is to show how request information gets
communicated to a CGI script. Note that POST data must be read from
standard input. In this particular case, posted data gets stored in the
variable `POST_DATA`. Your script may use a different method to read
POST content. Secondly, the `SCRIPT_EXEC` variable is not a CGI
standard. It is provided by this middleware and contains the entire
command line, including all arguments, with which the CGI script was
executed.

When a browser requests

    http://192.168.1.2:8080/show/weekly?mode=summary

the response looks like

    AUTH_TYPE         []
    CONTENT_LENGTH    []
    CONTENT_TYPE      []
    GATEWAY_INTERFACE [CGI/1.1]
    PATH_INFO         [/weekly]
    PATH_TRANSLATED   []
    POST_DATA         []
    QUERY_STRING      [mode=summary]
    REMOTE_ADDR       [192.168.1.35]
    REMOTE_HOST       [192.168.1.35]
    REMOTE_IDENT      []
    REMOTE_USER       []
    REQUEST_METHOD    [GET]
    SCRIPT_EXEC       [/usr/local/cgi-bin/report/gen]
    SCRIPT_NAME       [/show]
    SERVER_NAME       [192.168.1.2:8080]
    SERVER_PORT       [8080]
    SERVER_PROTOCOL   [HTTP/1.1]
    SERVER_SOFTWARE   [go]

When a client makes a POST request, such as with the following command

``` shell
wget -O - -q --post-data="city=San%20Francisco" http://192.168.1.2:8080/show/weekly?mode=summary
```

the response looks the same except for the following lines:

    CONTENT_LENGTH    [20]
    CONTENT_TYPE      [application/x-www-form-urlencoded]
    POST_DATA         [city=San%20Francisco]
    REQUEST_METHOD    [POST]

### Go Source Example

This small example demonstrates how to write a CGI program in Go. The
use of a bytes.Buffer makes it easy to report the content length in the
CGI header.

``` go
package main

import (
    "bytes"
    "fmt"
    "os"
    "time"
)

func main() {
    var buf bytes.Buffer

    fmt.Fprintf(&buf, "Server time at %s is %s\n",
        os.Getenv("SERVER_NAME"), time.Now().Format(time.RFC1123))
    fmt.Println("Content-type: text/plain")
    fmt.Printf("Content-Length: %d\n\n", buf.Len())
    buf.WriteTo(os.Stdout)
}
```

When this program is compiled and installed as
/usr/local/bin/servertime, the following directive in your Caddy file
will make it available:

``` caddy
cgi /servertime /usr/local/bin/servertime
```
