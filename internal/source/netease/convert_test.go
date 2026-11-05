package netease

import (
	"encoding/json"
	"errors"
	"testing"

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
	if !got.Playable {
		t.Fatal("a free song doesn't play")
	}

	// A VIP song, with no privilege to say otherwise, gives a preview only.
	vip := song
	vip.Fee = 1
	if g := track(vip); g.Playable || g.UnplayableReason == "" {
		t.Fatalf("a VIP song plays: %+v", g)
	}
	// One taken down says so.
	gone := song
	gone.Privilege = &api.Privilege{ID: song.ID, St: -1}
	if g := track(gone); g.Playable || g.UnplayableReason != "已下架" {
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
	if got.CommentThread != "A_DJ_1_99" || !got.Playable {
		t.Fatalf("thread %q, playable %v", got.CommentThread, got.Playable)
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
	if len(got.Aliases) != 2 || got.Aliases[1] != "Jay Chou" {
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
