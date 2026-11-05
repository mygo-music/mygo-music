package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// drain runs what the app posts until done says to stop: the main thread,
// for a test.
func drain(t *testing.T, a *app, done func() bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for !done() {
		select {
		case fn := <-a.posted:
			fn()
		case <-deadline:
			t.Fatal("what the test waited for did not happen")
		}
	}
}

// loginApp is an app whose source shows a QR code, then says who scanned it,
// then that they signed in, as the answers of the steps list it.
func loginApp(steps ...*source.Step) (*app, *int) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.posted = make(chan func(), 64)
	a.restored = true // the session of the last run is not under test
	polled := 0
	f := fakeOf(a)
	f.authBegin = func(string) (*source.Step, error) {
		return &source.Step{Type: source.StepQR, Token: "t", Content: "https://login.test/code", State: source.QRWaiting, Hint: "扫一扫", PollMS: 1}, nil
	}
	f.authPoll = func(token string) (*source.Step, error) {
		polled++
		st := steps[min(polled-1, len(steps)-1)]
		return st, nil
	}
	return a, &polled
}

// Signing in: the code is shown, someone scans it and the dialog says who,
// then they confirm and the app is signed in as them. Nothing tested the
// login before it was the source's to say.
func TestLoginFlow(t *testing.T) {
	defer func(f func(int) ([]netease.Play, error)) { recentPlays = f }(recentPlays)
	recentPlays = func(int) ([]netease.Play, error) { return nil, nil }

	a, _ := loginApp(
		&source.Step{Type: source.StepQR, Token: "t", State: source.QRScanned, Hint: "请在手机上确认", User: &source.UserRef{Name: "扫码的人"}, PollMS: 1},
		&source.Step{Type: source.StepDone, User: &source.UserRef{ID: "5", Name: "某人"}},
	)
	a.openLogin()
	drain(t, a, func() bool { return a.login.state == source.QRScanned })
	if a.login.code == nil || a.login.scanner != "扫码的人" || a.login.hint != "请在手机上确认" {
		t.Fatalf("scanned, the dialog has %+v", a.login)
	}
	tt := ui.NewTester(a.view, 1000, 700)
	tt.Frame()
	if !tt.HasText("扫码的人") || !tt.HasText("请在手机上确认") {
		t.Fatal("the dialog doesn't say who scanned the code, and what to do")
	}

	drain(t, a, func() bool { return a.login.finished })
	if a.profile == nil || a.profile.UserID != 5 || a.profile.Nickname != "某人" {
		t.Fatalf("signed in as %+v", a.profile)
	}
}

// An expired code stops the asking, and shows it can be refreshed; a source
// that refuses says why in its own words, and a network that fails is tried
// again.
func TestLoginExpiresAndRefuses(t *testing.T) {
	a, polled := loginApp(&source.Step{Type: source.StepQR, Token: "t", State: source.QRExpired, Hint: "二维码已失效", PollMS: 1})
	a.openLogin()
	drain(t, a, func() bool { return a.login.state == source.QRExpired })
	time.Sleep(30 * time.Millisecond) // the poll would go on, if it were to
	for len(a.posted) > 0 {
		(<-a.posted)()
	}
	if *polled != 1 {
		t.Fatalf("asked %d times, and kept asking after the code expired", *polled)
	}

	b := newTestApp()
	b.posted = make(chan func(), 64)
	calls := 0
	fakeOf(b).authBegin = func(string) (*source.Step, error) {
		return &source.Step{Type: source.StepQR, Token: "t", State: source.QRWaiting, PollMS: 1}, nil
	}
	fakeOf(b).authPoll = func(string) (*source.Step, error) {
		calls++
		if calls == 1 {
			return nil, source.Wrap(source.CodeNetwork, errOffline) // the network blips
		}
		return nil, source.Fail(source.CodeInternal, "账号被限制")
	}
	b.openLogin()
	drain(t, b, func() bool { return b.login.err != "" })
	if b.login.err != "账号被限制" || calls != 2 {
		t.Fatalf("error %q after %d asks, want the source's words after a retry", b.login.err, calls)
	}
}
