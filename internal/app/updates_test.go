package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A release is new only when its number is higher; a prerelease comes
// before its release, and a version that is no number, a development
// build's, is new when it differs.
func TestIsNewer(t *testing.T) {
	for _, c := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.1.26", "0.1.25", true},
		{"0.1.25", "0.1.25", false},
		{"v0.1.24", "0.1.25", false},
		{"v0.2.0", "0.1.99", true},
		{"v1.0.0", "0.9.9", true},
		{"v0.1.10", "0.1.9", true}, // numbers, not text
		{"v0.2.0", "0.2.0-beta.1", true},
		{"v0.2.0-beta.2", "0.2.0", false},
		{"v0.2.0", "dev", true},
	} {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

// 检查更新 in the settings looks for the latest release, once however many
// times it is asked while the look is under way.
func TestCheckForUpdatesFromSettings(t *testing.T) {
	asked, gate := make(chan struct{}, 4), make(chan struct{})
	real := latestRelease
	latestRelease = func() (*release, error) {
		asked <- struct{}{}
		<-gate
		return nil, errNoRelease
	}
	defer func() { latestRelease = real }()
	defer close(gate)

	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.settingsView, 560, 2000)
	tt.Frame()
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the release was not looked up")
	}
	a.checkUpdates()
	a.checkUpdates()
	select {
	case <-asked:
		t.Fatal("looked up again while looking")
	case <-time.After(50 * time.Millisecond):
	}
}

// A newer release is offered with a button that opens its page; the version
// running, or none published yet, offers nothing.
func TestOfferNewRelease(t *testing.T) {
	realVersion, realOpen := appVersion, openLink
	defer func() { appVersion, openLink = realVersion, realOpen }()
	appVersion = func() string { return "0.1.25" }
	var opened string
	openLink = func(url string) error { opened = url; return nil }

	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.settingsView, 560, 2000)
	tt.Frame()

	a.updateChecked(&release{Tag: "v0.1.26", Notes: "- 修好了一个问题\n", URL: "https://example.test/r"}, nil)
	tt.Frame()
	if err := tt.Click("前往下载"); err != nil {
		t.Fatal(err)
	}
	if opened != "https://example.test/r" {
		t.Fatalf("opened %q", opened)
	}

	for _, answer := range []struct {
		rel *release
		err error
	}{
		{&release{Tag: "v0.1.25", URL: "https://example.test/same"}, nil},
		{nil, errNoRelease},
	} {
		a.updateChecked(answer.rel, answer.err)
		tt.Frame()
		if _, ok := tt.Find("前往下载"); ok {
			t.Fatalf("offered a download after %v", answer)
		}
	}
}

// The latest release is read from what GitHub answers, a 404 meaning none
// has been published, and the request names the app, as GitHub requires.
func TestFetchLatestRelease(t *testing.T) {
	var agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get("User-Agent")
		if r.URL.Path == "/none" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"tag_name":"v0.2.0","body":"- 新功能","html_url":"https://example.test/r","draft":false}`))
	}))
	defer srv.Close()
	realAPI, realVersion := releaseAPI, appVersion
	defer func() { releaseAPI, appVersion = realAPI, realVersion }()
	appVersion = func() string { return "0.1.25" }

	releaseAPI = srv.URL + "/latest"
	rel, err := fetchLatestRelease()
	if err != nil || rel.Tag != "v0.2.0" || rel.URL != "https://example.test/r" || rel.Notes != "- 新功能" {
		t.Fatalf("release %+v, error %v", rel, err)
	}
	if agent != "MyGo-Music/0.1.25" {
		t.Fatalf("User-Agent %q", agent)
	}

	releaseAPI = srv.URL + "/none"
	if _, err := fetchLatestRelease(); !errors.Is(err, errNoRelease) {
		t.Fatalf("error %v, want none published", err)
	}
}
