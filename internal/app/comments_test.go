package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// The count on the comments' tab holds the last song's while the next
// one's loads, for it to roll from one to the other, and goes once the
// next song turns out to have none.
func TestCommentCountHoldsWhileLoading(t *testing.T) {
	a, tt, _ := coverApp(t)
	a.comments = commentState{track: 2, loaded: true, total: 128}
	tt.Frame()
	a.pl.Next()
	a.comments = commentState{track: 3, loading: true}
	tt.Frame()
	if _, ok := tt.Find("评论 128"); !ok {
		t.Fatal("the count went as the next song's loads")
	}
	a.comments.loading, a.comments.loaded, a.comments.total = false, true, 96
	tt.Frame()
	if _, ok := tt.Find("评论 96"); !ok {
		t.Error("the count did not change once loaded")
	}
	a.pl.Next()
	a.comments = commentState{track: 1, loaded: true}
	tt.Advance(time.Second)
	tt.Frame()
	if _, ok := tt.Find("评论"); !ok {
		t.Error("a song without comments shows a count")
	}
}

// Now Playing shows an episode's comments, the episode's and not its
// audio's, under a tab of their own beside what it is about, which stays
// chosen as the frames go.
func TestEpisodeComments(t *testing.T) {
	asked := make(chan string, 4)
	a := newTestApp()
	fakeOf(a).comments = func(thread, cursor string, limit int) (*source.Comments, error) {
		asked <- thread
		return &source.Comments{}, nil
	}
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 13, Name: "第一期", ProgramID: 3, PodcastID: 5, Duration: 600000}})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	if thread := wait(t, asked, "the comments"); thread != "A_DJ_1_3" {
		t.Fatalf("asked for the comments of %s", thread)
	}
	if a.npPane != paneProgram {
		t.Fatalf("opened on pane %d", a.npPane)
	}
	if err := tt.Click("评论"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.npPane != paneComments {
		t.Errorf("on pane %d", a.npPane)
	}
}

// 赞 on a comment likes it at once, counting the listener in where it is
// hot and where it is latest, and asks NetEase to; again, it unlikes it.
func TestLikeComment(t *testing.T) {
	type call struct {
		thread string
		id     string
		like   bool
	}
	calls := make(chan call, 4)
	a, tt, _ := coverApp(t)
	fakeOf(a).likeComment = func(thread, id string, like bool) error { calls <- call{thread, id, like}; return nil }
	a.profile = &netease.Profile{UserID: 1}
	cm := source.Comment{ID: "9", Text: "好听", LikeCount: 41}
	a.comments = commentState{track: 2, thread: "R_SO_4_2", loaded: true, total: 1, hot: []source.Comment{cm}, latest: []source.Comment{cm}}
	a.npPane = paneComments
	tt.Frame()
	if err := tt.Click("赞"); err != nil {
		t.Fatal(err)
	}
	if c := wait(t, calls, "the like"); c != (call{"R_SO_4_2", "9", true}) {
		t.Errorf("asked NetEase for %+v", c)
	}
	for _, cm := range []source.Comment{a.comments.hot[0], a.comments.latest[0]} {
		if !cm.Liked || cm.LikeCount != 42 {
			t.Fatalf("liked %v by %d", cm.Liked, cm.LikeCount)
		}
	}
	tt.Frame()
	if err := tt.Click("取消赞"); err != nil {
		t.Fatal(err)
	}
	if c := wait(t, calls, "the unlike"); c.like {
		t.Error("liked again")
	}
	for _, cm := range []source.Comment{a.comments.hot[0], a.comments.latest[0]} {
		if cm.Liked || cm.LikeCount != 41 {
			t.Errorf("unliked: liked %v by %d", cm.Liked, cm.LikeCount)
		}
	}
}

// Comments page on from the cursor the source gave, not one the app works
// out: the hot ones come with the first page only, a comment that slid onto
// the next page is not listed twice, and the end is where the source gives
// no cursor.
func TestCommentPaging(t *testing.T) {
	a := newTestApp()
	a.posted = make(chan func(), 16)
	var cursors []string
	fakeOf(a).comments = func(thread, cursor string, limit int) (*source.Comments, error) {
		cursors = append(cursors, cursor)
		if cursor == "" {
			return &source.Comments{Hot: []source.Comment{{ID: "h"}}, Items: []source.Comment{{ID: "1"}, {ID: "2"}}, Total: 3, Next: "after-2"}, nil
		}
		// New comments pushed 2 onto this page too.
		return &source.Comments{Items: []source.Comment{{ID: "2"}, {ID: "3"}}, Total: 3}, nil
	}
	a.comments = commentState{track: 1, thread: "R_SO_4_1"}
	a.loadComments()
	drain(t, a, func() bool { return a.comments.loaded })
	if !a.comments.more || len(a.comments.hot) != 1 || len(a.comments.latest) != 2 {
		t.Fatalf("first page: more %v, %d hot, %d latest", a.comments.more, len(a.comments.hot), len(a.comments.latest))
	}
	a.loadComments()
	drain(t, a, func() bool { return !a.comments.loading && len(a.comments.latest) > 2 })
	var ids []string
	for _, cm := range a.comments.latest {
		ids = append(ids, cm.ID)
	}
	if want := []string{"1", "2", "3"}; len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("listed %v, want %v", ids, want)
	}
	if a.comments.more || len(a.comments.hot) != 1 {
		t.Fatalf("after the last page: more %v, %d hot", a.comments.more, len(a.comments.hot))
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "after-2" {
		t.Fatalf("asked from %q, want the start, then the cursor the source gave", cursors)
	}
}
