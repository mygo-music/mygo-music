// Package fetch gets the pictures the app shows: covers, avatars, the things
// a source's addresses point at. It asks as a browser does and sends no
// cookies, whatever the source's session is.
package fetch

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// UserAgent is what the pictures' servers are told: they serve a browser.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"

// maxBytes is the most a picture may be, so that a wrong address can't fill
// the memory.
const maxBytes = 32 << 20

// The client has no cookie jar.
var client = &http.Client{Timeout: 20 * time.Second}

// Get returns what is at url, an http or https address.
func Get(url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, fmt.Errorf("fetch: not an http address: %.40q", url)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBytes {
		return nil, errors.New("fetch: too big for a picture")
	}
	return b, nil
}
