package source

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// A track as the specification's example gives it decodes, and plays: a
// track says it doesn't, or it does.
func TestDecodeTrack(t *testing.T) {
	var tr Track
	err := json.Unmarshal([]byte(`{
		"type": "track", "id": "186016", "title": "晴天",
		"artists": [{"id": "6452", "name": "周杰伦"}],
		"album": {"id": "18905", "name": "叶惠美", "cover": "https://x.test/a.jpg?param={size}y{size}"},
		"duration_ms": 269000, "comment_thread": "R_SO_4_186016"
	}`), &tr)
	if err != nil {
		t.Fatal(err)
	}
	if tr.ID != "186016" || tr.Title != "晴天" || tr.ArtistNames() != "周杰伦" || tr.Length().Seconds() != 269 {
		t.Fatalf("decoded %+v", tr)
	}
	if !tr.CanPlay() {
		t.Fatal("a track that doesn't say otherwise must play")
	}
	if got := Sized(tr.CoverURL(), 300); got != "https://x.test/a.jpg?param=300y300" {
		t.Fatalf("cover %q", got)
	}

	var no Track
	if err := json.Unmarshal([]byte(`{"id":"1","title":"x","playable":false}`), &no); err != nil || no.CanPlay() {
		t.Fatalf("playable:false came back %v, %v", no.CanPlay(), err)
	}
}

// Items of every kind come back as they went, each by its "type", and one
// of a kind this version doesn't know is skipped over, not an error: a
// newer plugin may send more kinds.
func TestItemJSON(t *testing.T) {
	in := []Item{
		{Type: TypeTrack, Track: &Track{ID: "1", Title: "a"}},
		{Type: TypePlaylist, Playlist: &Playlist{ID: "2", Title: "b", Role: RoleLiked}},
		{Type: TypePodcast, Podcast: &Podcast{ID: "3", Title: "c"}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out []Item
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0].Track.ID != "1" || !out[1].Playlist.IsLiked() || out[2].Podcast.Title != "c" {
		t.Fatalf("came back %s", b)
	}

	var mixed []Item
	if err := json.Unmarshal([]byte(`[{"type":"hologram","id":"9"},{"type":"user","id":"4","name":"d"}]`), &mixed); err != nil {
		t.Fatal(err)
	}
	if !mixed[0].Empty() || mixed[1].User.Name != "d" {
		t.Fatalf("mixed %+v", mixed)
	}
}

// An error matches the one of its code, however it was made and wrapped.
func TestErrorIs(t *testing.T) {
	err := Wrap(CodeNetwork, errors.New("connection reset"))
	if !errors.Is(err, ErrNetwork) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("%v", err)
	}
	// Wrapping what is a source error already keeps its code.
	if again := Wrap(CodeInternal, Fail(CodeUnavailable, "下架")); !errors.Is(again, ErrUnavailable) {
		t.Fatalf("%v", again)
	}
	if Wrap(CodeNetwork, nil) != nil {
		t.Fatal("nil became an error")
	}
	// A nil target is no match, and no panic.
	if errors.Is(err, (*Error)(nil)) {
		t.Fatal("matched a nil error")
	}
}

// A track made in Go plays, as one that says nothing of it in JSON does; a
// null leaves a track as it is; and a type that holds a track keeps its own
// fields, as a snapshot of the queue does.
func TestTrackPlaysByDefault(t *testing.T) {
	if !(&Track{}).CanPlay() {
		t.Fatal("a track made in Go doesn't play")
	}
	var tr Track
	tr.ID = "7"
	tr.SetUnplayable("下架")
	if tr.CanPlay() || tr.UnplayableReason != "下架" {
		t.Fatalf("SetUnplayable: %+v", tr)
	}
	if err := json.Unmarshal([]byte(`null`), &tr); err != nil || tr.ID != "7" || tr.CanPlay() {
		t.Fatalf("null changed the track: %+v, %v", tr, err)
	}
	var snap struct {
		Track
		SavedAt int64 `json:"saved_at"`
	}
	if err := json.Unmarshal([]byte(`{"id":"1","title":"x","saved_at":42}`), &snap); err != nil || snap.SavedAt != 42 || snap.ID != "1" {
		t.Fatalf("a snapshot lost a field: %+v, %v", snap, err)
	}
	// Nothing to say of the artists is nothing in JSON, not null.
	b, _ := json.Marshal(Track{ID: "1"})
	if strings.Contains(string(b), "null") {
		t.Fatalf("encoded %s", b)
	}
}

type fakeSource struct{ info Info }

func (f *fakeSource) Info() *Info { return &f.info }

type fakeSearcher struct{ fakeSource }

func (*fakeSearcher) Search(ctx context.Context, q SearchQuery) (*Page[Item], error) { return nil, nil }

// The source shown is the first added until another is chosen, and one
// removed gives way to the next.
func TestRegistryActive(t *testing.T) {
	a, b := &fakeSource{Info{ID: "a"}}, &fakeSource{Info{ID: "b"}}
	r := NewRegistry(a, b)
	if r.Active() != Source(a) {
		t.Fatal("the first added isn't shown")
	}
	if !r.SetActive("b") || r.Active() != Source(b) || r.SetActive("c") {
		t.Fatal("choosing a source")
	}
	r.Remove("b")
	if r.Active() != Source(a) {
		t.Fatal("removing the shown source left none")
	}
	r.Remove("a")
	if r.Active() != nil {
		t.Fatal("a source from nothing")
	}
}

// What a source can do is asked of it with As, and declared in its Info.
func TestAsCapability(t *testing.T) {
	var s Source = &fakeSearcher{fakeSource{Info{ID: "s", Capabilities: []string{CapSearch}}}}
	if _, ok := As[Searcher](s); !ok {
		t.Fatal("a searcher isn't one")
	}
	if _, ok := As[Streamer](s); ok {
		t.Fatal("a searcher streams")
	}
	if !s.Info().Has(CapSearch) || s.Info().Has(CapFeed) {
		t.Fatal("capabilities")
	}
}
