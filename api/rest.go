package api

import (
	"net/http"

	"runtime.link/api/internal/rest"
	"runtime.link/std"
)

/*
# REST Transport

	var API struct {
		api.Specification `rest:"http://api.example.com/v1"`

		Echo func(message string) string `rest:"GET /echo?message=%v"`
	}

When Echo is called, it calls the function over HTTPS/REST to
'https://api.example.com/v1/echo' and returns the result (if there
is an error and the function doesn't return one, it will panic).

The REST transport will be served by the [api.Handler] when
the Content-Type of the request is 'application/json' and the
API has suitable rest tags.

	API.Echo = func(message string) string { return message }
	api.ListenAndServe(":"+os.Getenv("PORT"), &API)

This starts a local HTTP server and listens on PORT
for requests to /echo and responds to these requests with the
defined Echo function. Arguments and Results are automatically
converted to JSON.

# Tags

Each API function can have a rest tag that formats
function arguments (%v) to query parameters.
Each tag must follow the space-seperated pattern:

	GET /path/to/endpoint/{object=%v}?query=%v (argument,mapping,rules) result,mapping,rules
	[METHOD] [PATH] (ARGUMENT_RULES) RESULT_RULES

It begins with a METHOD, then with a PATH format string
that descibes how the function arguments are mapped onto
the HTTP path & query. This follows standard fmt rules.

The path can contain path expansion parameters {name=%v} or
ordinary format parameters %v (similar to the fmt package).
Think of the arguments of the function as the parameters that
get passed to a printf call. Imagine it working like this:

	http.Get(fmt.Sprintf("/path/with/%v?query=%v", value, query))

If a path or query expansion parameter omits a format parameter,
the value will be considered to nested within a struct argument
and the name of the parameter will be used to look for the first
matching field in subsequent body structures. Either by field
name or by rest tag.

	POST /path/to/endpoint/{ID}
	{
		ID: "1234",
		Value: "something"
	}

ARGUMENT_RULES are optional, they are a comma separated list
of names to give the remaining arguments in the JSON body
of the request. By default, arguments are posted as an
array, however if there are ARGUMENT_RULES, the arguments
will be mapped into json fields of the name, matching the
argument's position.

	foo func(id int, value string) `rest:"POST /foo (id,value)"`
	foo(22, "Hello World") => {"id": 22, "value":"Hello World"}

RESULT_RULES are much like ARGUMENT_RULES, except they operate
on the results of the function instead of the arguments. They
map named json fields to the result values.

	getLatLong func() (float64, float64) `rest:"GET /latlong latitude,longitude"`
	{"latitude": 12.2, "longitude": 15.0} => lat, lon := getLatLong()

# Response Headers

In order to read and write HTTP headers in a request, an Authenticator should
be used. However to read and write response headers, result values can implement
the http.HeaderWriter and http.HeaderReader interfaces:

	type HeaderWriter interface {
		WriteHeadersHTTP(http.Header)
	}

	type HeaderReader interface {
		ReadHeadersHTTP(http.Header)
	}

If multiple result values implement these interfaces, they will be called in the order
they are returned. Here's an example:

	type ProfilePicture struct {
		io.ReadCloser
	}

	func (ProfilePicture) WriteHeadersHTTP(header http.Header) {
		header.Set("Content-Type", "image/png")
	}

	type API struct {
		api.Specification

		GetProfilePicture func() (ProfilePicture, error)
	}
*/
func REST(link string, access AccessController, structure std.Structure) (http.Handler, error) {
	return rest.Transport(link, access, structure)
}
