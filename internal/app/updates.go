package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// The app doesn't update itself. When the listener asks, in the settings or
// the menu, it looks up the latest release of its repository on GitHub, and
// if that is newer than the app, shows what changed and opens the release's
// page, where the downloads are.

// releaseAPI is where the latest published release, neither a draft nor a
// prerelease, is asked for. A variable for the tests.
var releaseAPI = "https://api.github.com/repos/mygo-music/mygo-music/releases/latest"

// release is a published release of the app.
type release struct {
	Tag   string `json:"tag_name"`
	Notes string `json:"body"`
	URL   string `json:"html_url"`
}

// errNoRelease says the repository has published no release yet, and
// errRateLimited that GitHub is not answering so many requests.
var (
	errNoRelease   = errors.New("no release published")
	errRateLimited = errors.New("GitHub is rate limiting")
)

// These look up the latest release, say which version runs, and open a link
// in the browser; tests replace them.
var (
	latestRelease = fetchLatestRelease
	appVersion    = func() string { return mygo.App.Version() }
	openLink      = mygo.Shell.OpenExternal
)

func fetchLatestRelease() (*release, error) {
	req, err := http.NewRequest("GET", releaseAPI, nil)
	if err != nil {
		return nil, err
	}
	// GitHub refuses requests without a User-Agent.
	req.Header.Set("User-Agent", "MyGo-Music/"+appVersion())
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, errNoRelease
	case res.StatusCode == http.StatusForbidden, res.StatusCode == http.StatusTooManyRequests:
		return nil, errRateLimited
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub answered %s", res.Status)
	}
	var rel release
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.Tag == "" {
		return nil, errors.New("GitHub's release has no tag")
	}
	return &rel, nil
}

// updateState is what the last look for a new version found.
type updateState struct {
	checking bool
	at       time.Time // when the last look that got an answer ended; zero before one
	found    *release  // the release newer than the app, if the last answer was one
	none     bool      // the last answer was that no release is published
	err      string    // why the last look failed; what was found before stays
}

// checkUpdates looks for a new version, unless a look is under way.
func (a *app) checkUpdates() {
	if a.update.checking {
		return
	}
	a.update.checking, a.update.err = true, ""
	a.invalidateSettings()
	go func() {
		rel, err := latestRelease()
		a.post(func() { a.updateChecked(rel, err) })
	}()
}

// updateChecked takes in what a look for a new version found. A look that
// fails leaves what an earlier one found.
func (a *app) updateChecked(rel *release, err error) {
	u := &a.update
	u.checking = false
	defer a.invalidateSettings()
	switch {
	case errors.Is(err, errRateLimited):
		u.err = "GitHub 请求过于频繁，请稍后再试"
		return
	case err != nil && !errors.Is(err, errNoRelease):
		log.Println("updates:", err)
		u.err = "无法连接 GitHub，请稍后再试"
		return
	}
	u.at, u.err, u.found = time.Now(), "", nil
	u.none = errors.Is(err, errNoRelease)
	if err == nil && isNewer(rel.Tag, appVersion()) {
		u.found = rel
	}
}

// invalidateSettings redraws the settings window, which shows the state of
// the look: post and the menus redraw only the main window.
func (a *app) invalidateSettings() {
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
}

// checkUpdatesFromMenu shows the settings, where the answer is, and asks.
func (a *app) checkUpdatesFromMenu() {
	a.openSettings()
	a.checkUpdates()
}

// openRelease opens the page of a release in the browser.
func (a *app) openRelease(rel *release) {
	if err := openLink(rel.URL); err != nil {
		a.toast = "无法打开浏览器"
	}
}

// updateStatus says what the last check found, for the settings.
func (a *app) updateStatus() string {
	u := &a.update
	switch {
	case u.checking:
		return "正在检查…"
	case u.err != "":
		return u.err
	case u.found != nil:
		return "有新版本可用"
	case u.none:
		return "本仓库还没有发布过版本"
	case !u.at.IsZero():
		return "已是最新版本（" + commentTime(u.at, time.Now()) + "检查）"
	}
	return "需要时检查 GitHub 上是否有新版本，应用不会自动更新"
}

// releaseSummary is the first lines of a release's notes, which are its
// changelog's section: a line for each change.
func releaseSummary(notes string) string {
	var lines []string
	plain := strings.NewReplacer("**", "", "`", "")
	for _, line := range strings.Split(plain.Replace(notes), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	const most = 6
	if len(lines) > most {
		lines = append(lines[:most], "…")
	}
	return strings.Join(lines, "\n")
}

// version is a version number read as "1.2.3", with or without a "v", and
// maybe a "-beta.1" or "+build" after it.
type version struct {
	n   [3]int
	pre bool // a prerelease, which comes before its release
}

func parseVersion(s string) (v version, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s, v.pre = s[:i], true
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v.n[i] = n
	}
	return v, true
}

// isNewer reports whether version latest comes after current. A latest that
// is no version number is no update; a current that isn't one, a
// development build's, is older than any release.
func isNewer(latest, current string) bool {
	l, ok1 := parseVersion(latest)
	c, ok2 := parseVersion(current)
	switch {
	case !ok1:
		return false
	case !ok2:
		return true
	}
	for i := range l.n {
		if l.n[i] != c.n[i] {
			return l.n[i] > c.n[i]
		}
	}
	return c.pre && !l.pre
}
