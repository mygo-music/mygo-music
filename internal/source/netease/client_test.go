package netease

import (
	"context"
	"encoding/json"
	"testing"

	api "github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// swap replaces a call to the service until the returned function puts it
// back, as the tests of the app replace theirs.
func swap[T any](p *T, v T) func() {
	old := *p
	*p = v
	return func() { *p = old }
}

// offline keeps the tests off the keychain and the network.
func offline(t *testing.T) *Source {
	t.Helper()
	t.Cleanup(swap(&apiRestoreSession, func() bool { return false }))
	t.Cleanup(swap(&apiClearSession, func() {}))
	return &Source{}
}

// An artist's events page on from the time of the last, none for the first,
// and end when a page doesn't start earlier than the one before, which would
// load forever.
func TestEventsCursor(t *testing.T) {
	var asked []int64
	defer swap(&apiEvents, func(uid int64, limit int, before int64) ([]api.Event, int64, bool, error) {
		asked = append(asked, before)
		return []api.Event{{ID: 1}}, 500, true, nil
	})()
	s := offline(t)
	artist := source.EntityRef{Type: source.TypeArtist, ID: "6452", Extra: json.RawMessage(`{"account_id":123}`)}

	first, err := s.LoadTab(context.Background(), artist, tabEvents, "")
	if err != nil || first.Next != "500" || len(first.Items) != 1 || first.Items[0].Post == nil {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	second, err := s.LoadTab(context.Background(), artist, tabEvents, first.Next)
	if err != nil || second.Next != "" {
		t.Fatalf("a page that starts no earlier went on: %+v, %v", second, err)
	}
	if len(asked) != 2 || asked[0] != -1 || asked[1] != 500 {
		t.Fatalf("asked from %v, want the latest then 500", asked)
	}

	// An artist with no account of their own has no events.
	none, err := s.LoadTab(context.Background(), source.EntityRef{Type: source.TypeArtist, ID: "1"}, tabEvents, "")
	if err != nil || len(none.Items) != 0 {
		t.Fatalf("events of an artist with no account: %+v, %v", none, err)
	}
}

// The hot podcasts go on from where the last page ended.
func TestHotPodcastPaging(t *testing.T) {
	var limit, offset int
	defer swap(&apiHotPodcasts, func(l, o int) ([]api.Podcast, bool, error) {
		limit, offset = l, o
		return make([]api.Podcast, podcastsPage), true, nil
	})()
	sec, err := offline(t).PodcastSection(context.Background(), "hot", "30")
	if err != nil || limit != podcastsPage || offset != 30 || sec.Next != "60" || !sec.More {
		t.Fatalf("limit %d offset %d, section %+v, %v", limit, offset, sec, err)
	}
}

// NetEase answers for episodes by their own IDs, and the app asks about
// tracks: the answer comes back keyed by the track.
func TestLikedStateByTrackID(t *testing.T) {
	var asked []int64
	defer swap(&apiProgramsLiked, func(ids []int64) (map[int64]bool, error) {
		asked = ids
		return map[int64]bool{99: true, 100: false}, nil
	})()
	ep := func(audio string, program string) source.Track {
		return source.Track{ID: audio, Kind: source.KindEpisode, Episode: &source.Episode{ID: program}}
	}
	got, err := offline(t).LikedState(context.Background(), []source.Track{ep("555", "99"), ep("556", "100"), {ID: "7"}})
	if err != nil || !got["555"] || got["556"] {
		t.Fatalf("liked state %v, %v", got, err)
	}
	if len(asked) != 2 || asked[0] != 99 || asked[1] != 100 {
		t.Fatalf("asked of episodes %v", asked)
	}
}

// A like of an episode goes to the episode, a like of a song to the song.
func TestLikeGoesToEpisodeOrSong(t *testing.T) {
	var program, song int64
	defer swap(&apiLikeProgram, func(id int64, like bool) error { program = id; return nil })()
	defer swap(&apiLike, func(id int64, like bool) error { song = id; return nil })()
	s := offline(t)
	ep := source.Track{ID: "555", Kind: source.KindEpisode, Episode: &source.Episode{ID: "99"}}
	if err := s.Like(context.Background(), &ep, true); err != nil || program != 99 || song != 0 {
		t.Fatalf("episode: program %d, song %d, %v", program, song, err)
	}
	if err := s.Like(context.Background(), &source.Track{ID: "7"}, true); err != nil || song != 7 {
		t.Fatalf("song: %d, %v", song, err)
	}
}

// The QR login is waiting, scanned by someone, expired, or confirmed, which
// signs the listener in.
func TestQRStates(t *testing.T) {
	var code int
	var nick string
	defer swap(&apiQRCheck, func(key string) (int, string, string, string, error) {
		return code, "", nick, "http://a.test/me.jpg", nil
	})()
	defer swap(&apiSaveSession, func() error { return nil })()
	defer swap(&apiAccount, func() (*api.Profile, error) { return &api.Profile{UserID: 5, Nickname: "某人"}, nil })()
	s := offline(t)
	ctx := context.Background()

	code = api.QRWaiting
	if st, err := s.AuthPoll(ctx, "k"); err != nil || st.Type != source.StepQR || st.State != source.QRWaiting || st.Hint == "" {
		t.Fatalf("waiting: %+v, %v", st, err)
	}
	code, nick = api.QRScanned, "扫码的人"
	if st, err := s.AuthPoll(ctx, "k"); err != nil || st.State != source.QRScanned || st.User == nil || st.User.Name != "扫码的人" {
		t.Fatalf("scanned: %+v, %v", st, err)
	}
	code = api.QRExpired
	if st, err := s.AuthPoll(ctx, "k"); err != nil || st.State != source.QRExpired {
		t.Fatalf("expired: %+v, %v", st, err)
	}
	code = api.QRConfirmed
	st, err := s.AuthPoll(ctx, "k")
	if err != nil || st.Type != source.StepDone || st.User == nil || st.User.Name != "某人" {
		t.Fatalf("confirmed: %+v, %v", st, err)
	}
	// Any other code is NetEase refusing, with its own words.
	code = 8821
	if _, err := s.AuthPoll(ctx, "k"); err == nil {
		t.Fatal("a refusal passed for a state")
	}
}

// A picture sized through the template is the picture the app has always
// asked the CDN for.
func TestSizedMatchesTheClient(t *testing.T) {
	for _, pic := range []string{"http://p1.music.126.net/a.jpg", "https://p1.music.126.net/a.jpg"} {
		for _, n := range []int{60, 300, 600} {
			if got, want := source.Sized(image(pic), n), api.Sized(pic, n); got != want {
				t.Errorf("Sized(%q, %d) = %q, want %q", pic, n, got, want)
			}
		}
	}
	if image("") != "" {
		t.Error("an empty picture is not a template")
	}
}

// Suggestions come in the order the service ranks their kinds, then those it
// left out, and a kind with nothing to suggest has no group.
func TestSuggestOrder(t *testing.T) {
	full := api.Suggestions{
		Songs:     []api.Track{{ID: 1, Name: "歌"}},
		Artists:   []api.Artist{{ID: 2, Name: "人"}},
		Albums:    []api.AlbumProfile{{Album: api.Album{ID: 3, Name: "辑"}}},
		Playlists: []api.Playlist{{ID: 4, Name: "单"}},
		Order:     []string{"artists"},
	}
	types := func(res api.Suggestions) []string {
		defer swap(&apiSearchSuggest, func(string) (api.Suggestions, error) { return res, nil })()
		groups, err := offline(t).Suggest(context.Background(), "x")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, g := range groups {
			out = append(out, g.Type)
		}
		return out
	}
	if got := types(full); len(got) != 4 || got[0] != source.TypeArtist || got[1] != source.TypeTrack || got[2] != source.TypeAlbum || got[3] != source.TypePlaylist {
		t.Fatalf("order %v, want artists first, then songs, albums, playlists", got)
	}
	onlySongs := api.Suggestions{Songs: full.Songs, Order: []string{"albums"}}
	if got := types(onlySongs); len(got) != 1 || got[0] != source.TypeTrack {
		t.Fatalf("a kind with nothing in it got a group: %v", got)
	}
}
