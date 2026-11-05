package netease

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	api "github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// A track converts with its numbers as strings and its picture as the
// template the app sizes, and plays only as NetEase lets it.
func TestTrackConversion(t *testing.T) {
	song := api.Track{
		ID: 186016, Name: "晴天", Duration: 269000,
		Artists: []api.Artist{{ID: 6452, Name: "周杰伦"}},
		Album:   api.Album{ID: 18905, Name: "叶惠美", PicURL: "http://p1.music.126.net/a.jpg"},
	}
	got := track(song)
	if got.ID != "186016" || got.Plugin != PluginID || got.Title != "晴天" || got.Artists[0].ID != "6452" || got.DurationMS != 269000 {
		t.Fatalf("converted %+v", got)
	}
	if got.Album.Cover != "https://p1.music.126.net/a.jpg?param={size}y{size}" {
		t.Fatalf("cover %q", got.Album.Cover)
	}
	if got.CommentThread != "R_SO_4_186016" || got.URL != "https://music.163.com/song?id=186016" {
		t.Fatalf("thread %q, url %q", got.CommentThread, got.URL)
	}
	if !got.CanPlay() {
		t.Fatal("a free song doesn't play")
	}

	// A VIP song, with no privilege to say otherwise, gives a preview only.
	vip := song
	vip.Fee = 1
	if g := track(vip); g.CanPlay() || g.UnplayableReason == "" {
		t.Fatalf("a VIP song plays: %+v", g)
	}
	// One taken down says so.
	gone := song
	gone.Privilege = &api.Privilege{ID: song.ID, St: -1}
	if g := track(gone); g.CanPlay() || g.UnplayableReason == "" {
		t.Fatalf("a song taken down: %+v", g)
	}
}

// An episode plays as the track of its audio, with the episode's own ID,
// its podcast and its comments thread, which is the episode's, not the
// audio's.
func TestEpisodeConversion(t *testing.T) {
	tr := api.Track{ID: 555, Name: "第一期", ProgramID: 99, PodcastID: 7, Description: "说了什么", Album: api.Album{Name: "某播客"}}
	got := track(tr)
	if !got.IsEpisode() || got.ID != "555" || got.Episode.ID != "99" || got.Episode.Podcast.ID != "7" || got.Episode.Podcast.Title != "某播客" {
		t.Fatalf("episode %+v / %+v", got, got.Episode)
	}
	if got.CommentThread != "A_DJ_1_99" || !got.CanPlay() {
		t.Fatalf("thread %q, plays %v", got.CommentThread, got.CanPlay())
	}
}

// IDs go to strings and back, and one that isn't a number is the caller's
// error, not a crash.
func TestIDs(t *testing.T) {
	if id(0) != "" || id(42) != "42" {
		t.Fatal("id")
	}
	if n, err := number("42"); err != nil || n != 42 {
		t.Fatalf("number: %d, %v", n, err)
	}
	if _, err := number("abc"); err == nil || !isInvalid(err) {
		t.Fatalf("a bad ID gave %v", err)
	}
}

func isInvalid(err error) bool {
	return errors.Is(err, source.ErrInvalidArgument)
}

// A liked-songs playlist and a private one are marked as the app reads
// them, and a chart carries the lines of its card.
func TestPlaylistConversion(t *testing.T) {
	liked := api.Playlist{ID: 1, Name: "我喜欢的音乐", SpecialType: 5, Privacy: 10, CoverURL: "http://c.test/x.jpg"}
	got := playlist(liked)
	if !got.IsLiked() || !got.Private || got.Cover != "https://c.test/x.jpg?param={size}y{size}" {
		t.Fatalf("liked %+v", got)
	}
	chart := api.Playlist{ID: 2, Name: "飙升榜", Preview: []api.ChartLine{{First: "歌", Second: "人"}}}
	if c := playlist(chart); len(c.Preview) != 1 || c.Preview[0].Title != "歌" || c.Preview[0].Subtitle != "人" {
		t.Fatalf("chart %+v", c)
	}
}

// An artist keeps the account their events are posted by in their extra,
// which the page of events asks for, and the aliases take in the
// translated name.
func TestArtistConversion(t *testing.T) {
	ar := api.ArtistProfile{Artist: api.Artist{ID: 6452, Name: "周杰伦"}, Alias: []string{"Jay"}, Trans: "Jay Chou", AccountID: 123}
	got := artist(ar)
	var x artistExtra
	if err := json.Unmarshal(got.Extra, &x); err != nil || x.AccountID != 123 {
		t.Fatalf("extra %s, %v", got.Extra, err)
	}
	if len(got.Aliases) != 2 || got.Aliases[0] != "Jay Chou" || got.Aliases[1] != "Jay" {
		t.Fatalf("aliases %v", got.Aliases)
	}
	if none := artist(api.ArtistProfile{Artist: api.Artist{ID: 1}}); none.Extra != nil {
		t.Fatalf("extra %s for an artist with no account", none.Extra)
	}
}

// Word-timed lyrics become lines whose words carry their times, with the
// translation on each line; lines of plain LRC carry no words, for the app
// to spread.
func TestWordTimedLyrics(t *testing.T) {
	yrc := `[13010,4780](13010,660,0)从(13670,670,0)不(14340,30,0) (14370,480,0)主
[17790,1770](17790,170,0)I(17960,450,0) (18410,290,0)love`
	lines := wordTimed("", "[00:13.00]Never\n[00:17.80]我爱", yrc)
	if len(lines) != 2 {
		t.Fatalf("lines %+v", lines)
	}
	l := lines[0]
	if l.Text != "从不 主" || l.StartMS != 13010 || l.EndMS != 17790 || l.Translation != "Never" {
		t.Fatalf("line %+v", l)
	}
	if len(l.Words) != 3 || l.Words[2].StartMS != 14370 {
		t.Fatalf("words %+v", l.Words)
	}

	plain := wordTimed("[00:30.54]故事的小黄花", "", "")
	if len(plain) != 1 || len(plain[0].Words) != 0 {
		t.Fatalf("plain lines %+v", plain)
	}
}

// The cursor of a page is where the next starts, and is none at the end.
func TestPaging(t *testing.T) {
	if offsetOf("") != 0 || offsetOf("30") != 30 || offsetOf("x") != 0 || offsetOf("-5") != 0 {
		t.Fatal("offsetOf")
	}
	if next(0, 30, true) != "30" || next(30, 30, true) != "60" || next(0, 30, false) != "" || next(0, 0, true) != "" {
		t.Fatal("next")
	}
}

// An episode with no audio of its own can't play; the app disables its row.
func TestEpisodeWithoutAudio(t *testing.T) {
	got := track(api.Track{ID: 0, Name: "第二期", ProgramID: 100, PodcastID: 7})
	if got.CanPlay() || got.UnplayableReason == "" {
		t.Fatalf("an episode with no audio plays: %+v", got)
	}
}

// The service's own codes tell a listener who isn't signed in, a thing that
// isn't there and being asked too often from a failure to reach it, and a
// format the player doesn't decode is a song that can't play.
func TestErrorCodes(t *testing.T) {
	for _, c := range []struct {
		err  error
		want *source.Error
	}{
		{&api.APIError{What: "x", Code: api.CodeNotSignedIn}, source.ErrUnauthenticated},
		{&api.APIError{What: "x", Code: api.CodeNotFound}, source.ErrNotFound},
		{&api.APIError{What: "x", Code: api.CodeTooFrequent}, source.ErrRateLimited},
		{&api.APIError{What: "x", Code: 500}, source.ErrInternal},
		{errors.New("connection reset"), source.ErrNetwork},
	} {
		if got := failed(c.err); !errors.Is(got, c.want) {
			t.Errorf("%v became %v, want %v", c.err, got, c.want)
		}
	}
}

// The home page has its sections in the order the page shows them, with
// four charts of all, and the daily songs, which many listen to without
// signing in, with none of the playlists picked for the listener.
func TestHomeSections(t *testing.T) {
	charts := make([]api.Playlist, 9)
	for i := range charts {
		charts[i] = api.Playlist{ID: int64(i + 1), Name: "榜"}
	}
	pl := []api.Playlist{{ID: 100, Name: "歌单"}}
	songs := []api.Track{{ID: 1, Name: "a"}}
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, beijing)

	got := homeSections(songs, pl, pl, charts, now)
	var ids []string
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if want := []string{"daily", "recommended", "charts", "featured"}; !slices.Equal(ids, want) {
		t.Fatalf("sections %v, want %v", ids, want)
	}
	if c := got[2]; len(c.Items) != 4 || !c.More || c.Style != source.StyleChart {
		t.Fatalf("charts section: %d items, more %v", len(c.Items), c.More)
	}
	if d := got[0]; !d.PlayAll || d.Style != source.StyleHero || d.ExpiresMS == 0 {
		t.Fatalf("daily section %+v", d)
	}

	// Signed out: no playlists picked for the listener; and with few charts,
	// no page of the rest.
	got = homeSections(songs, nil, pl, charts[:3], now)
	if len(got) != 3 || got[1].ID != "charts" || got[1].More {
		t.Fatalf("signed out: %+v", got)
	}
	// Nothing loaded is no section, not an empty one.
	if got := homeSections(nil, nil, nil, nil, now); len(got) != 0 {
		t.Fatalf("empty home has %d sections", len(got))
	}
}

// The daily songs are picked anew at 6:00 Beijing time: the section says
// when, so the app loads it again.
func TestNextDailyAt(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 5, h, m, 0, 0, beijing) }
	for _, c := range []struct{ now, want time.Time }{
		{at(5, 59), at(6, 0)},
		{at(6, 0), at(6, 0).AddDate(0, 0, 1)}, // already picked: the next day's
		{at(23, 0), at(6, 0).AddDate(0, 0, 1)},
		// Another zone's clock is the same moment.
		{at(5, 0).In(time.UTC), at(6, 0)},
	} {
		if got := nextDailyAt(c.now); !got.Equal(c.want) {
			t.Errorf("nextDailyAt(%v) = %v, want %v", c.now, got, c.want)
		}
	}
}

// A user's page lists the playlists they made and made public: not those
// they saved of others, nor their private ones.
func TestPublicMine(t *testing.T) {
	mine := func(id int64, creator int64, privacy int) api.Playlist {
		p := api.Playlist{ID: id, Privacy: privacy}
		p.Creator.UserID = creator
		return p
	}
	got := publicMine([]api.Playlist{mine(1, 5, 0), mine(2, 9, 0), mine(3, 5, 10), mine(4, 5, 0)}, 5)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 4 {
		t.Fatalf("kept %+v", got)
	}
}

// Recent plays name where they were played, as the kind of device.
func TestDevice(t *testing.T) {
	for os, want := range map[string]string{"iphone": source.DevicePhone, "android": source.DevicePhone, "osx": source.DeviceDesktop, "pc": source.DeviceDesktop, "web": source.DeviceWeb, "toaster": ""} {
		if got := device(os); got != want {
			t.Errorf("device(%q) = %q, want %q", os, got, want)
		}
	}
}
