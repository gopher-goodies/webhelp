package webhelp

import (
	"net/http"
	"strings"
	"sync/atomic"

	"golang.org/x/net/context"
)

// DirMux is a Handler that mimics a directory. It mutates an incoming
// request's URL.Path to properly namespace handlers. This way a handler can
// assume it has the root of its section. If you want the original URL, use
// req.RequestURI (but don't modify it).
type DirMux map[string]Handler

func (d DirMux) ServeHTTP(ctx context.Context, w ResponseWriter,
	r *http.Request) error {
	dir, left := Shift(r.URL.Path)
	handler, ok := d[dir]
	if !ok {
		return ErrNotFound.New("resource: %#v", dir)
	}
	r.URL.Path = left
	return handler.ServeHTTP(ctx, w, r)
}

// Shift pulls the first directory out of the path and returns the remainder.
func Shift(path string) (dir, left string) {
	// slice off the first "/"s if they exists
	path = strings.TrimLeft(path, "/")

	if len(path) == 0 {
		return "", ""
	}

	// find the first '/' after the initial one
	split := strings.Index(path, "/")
	if split == -1 {
		return path, ""
	}
	return path[:split], path[split:]
}

// MethodMux is a Handler muxer that keys off of the given HTTP request method
type MethodMux map[string]Handler

func (m MethodMux) ServeHTTP(ctx context.Context, w ResponseWriter,
	r *http.Request) error {
	if handler, found := m[r.Method]; found {
		return handler.ServeHTTP(ctx, w, r)
	}
	return ErrMethodNotAllowed.New("bad method: %#v", r.Method)
}

// ArgMux is a way to pull off arbitrary path elements from an incoming URL.
// You'll need to create one with NewArgMux.
type ArgMux struct {
	id int64
}

var argMuxCounter int64

func NewArgMux() ArgMux {
	return ArgMux{id: atomic.AddInt64(&argMuxCounter, 1)}
}

// Shift takes a Handler and returns a new Handler that does additional request
// processing. When an incoming request is processed, the new Handler pulls the
// next path element off of the incoming request path and puts the value in the
// current Context. It then passes processing off to the wrapped Handler.
func (a ArgMux) Shift(h Handler) Handler {
	return HandlerFunc(func(ctx context.Context, w ResponseWriter,
		r *http.Request) error {
		var arg string
		arg, r.URL.Path = Shift(r.URL.Path)
		return h.ServeHTTP(context.WithValue(ctx, a, arg), w, r)
	})
}

// Get returns a stored value for the Arg from the Context, or "" if no value
// was found.
func (a ArgMux) Get(ctx context.Context) string {
	if val, ok := ctx.Value(a).(string); ok {
		return val
	}
	return ""
}

// ExactPath takes a Handler that returns a new Handler that doesn't accept any
// more path elements
func ExactPath(h Handler) Handler {
	return DirMux{"": h}
}

// ExactMethod takes a Handler and returns a new Handler that only works with
// the given HTTP method.
func ExactMethod(method string, h Handler) Handler {
	return MethodMux{method: h}
}

// ExactGet is simply ExactMethod but called with "GET" as the first argument.
func ExactGet(h Handler) Handler {
	return ExactMethod("GET", h)
}

// Exact is simply ExactGet and ExactPath called together.
func Exact(h Handler) Handler {
	return ExactGet(ExactPath(h))
}