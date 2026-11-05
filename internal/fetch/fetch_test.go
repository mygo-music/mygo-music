package fetch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A picture is asked for as a browser asks, with no cookie, however many
// were set: a source's session is not the picture server's business.
func TestGetSendsNoCookies(t *testing.T) {
	var agent string
	var cookies int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, cookies = r.Header.Get("User-Agent"), len(r.Cookies())
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "secret"})
		w.Write([]byte("pixels"))
	}))
	defer srv.Close()

	for range 2 { // the second asks after a cookie was set
		b, err := Get(srv.URL)
		if err != nil || string(b) != "pixels" {
			t.Fatalf("got %q, %v", b, err)
		}
	}
	if agent != UserAgent || cookies != 0 {
		t.Fatalf("asked with agent %q and %d cookies", agent, cookies)
	}
}

// An answer that isn't a picture is an error, as is an address that isn't
// http, and a picture too big to keep.
func TestGetRefuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/big":
			w.Write([]byte(strings.Repeat("x", maxBytes+1)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	for _, url := range []string{srv.URL + "/missing", srv.URL + "/big", "file:///etc/passwd", "ftp://x.test/a.jpg", ""} {
		if b, err := Get(url); err == nil {
			t.Errorf("Get(%.40q) = %d bytes, want an error", url, len(b))
		}
	}
}
