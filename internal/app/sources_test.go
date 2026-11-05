package app

import (
	"testing"

	"github.com/mygo-music/mygo-music/internal/source"
)

// bare is a source that declares nothing and can do nothing.
type bare struct{}

func (bare) Info() *source.Info { return &source.Info{ID: "bare", Name: "空来源"} }

// The app asks the source that shows for what it can do, and says what it is
// called; with no source, or one that can't, it gets no answer rather than a
// crash: tests build apps with none.
func TestActiveSource(t *testing.T) {
	none := &app{}
	if none.active() != nil || none.sourceName() == "" {
		t.Fatalf("an app with no sources: %v, %q", none.active(), none.sourceName())
	}
	if _, ok := capOf[source.Searcher](none); ok {
		t.Fatal("an app with no sources searches")
	}

	a := newTestApp()
	if a.sourceName() != "测试来源" {
		t.Fatalf("the name is %q", a.sourceName())
	}
	if _, ok := capOf[source.Searcher](a); !ok {
		t.Fatal("the fake can't search")
	}

	a.reg = source.NewRegistry(bare{})
	if _, ok := capOf[source.Searcher](a); ok || a.sourceName() != "空来源" {
		t.Fatalf("a source that does nothing: searches %v, called %q", ok, a.sourceName())
	}
}

// A call the test gave the fake no answer to fails, the way a network that
// is down does, so that the app shows its error and carries on.
func TestFakeSourceFailsWhatItIsNotGiven(t *testing.T) {
	a := newTestApp()
	s, _ := capOf[source.Searcher](a)
	if _, err := s.Search(t.Context(), source.SearchQuery{Query: "x"}); err == nil {
		t.Fatal("a search the test gave no answer to succeeded")
	}
	fakeOf(a).search = func(q source.SearchQuery) (*source.Page[source.Item], error) {
		return &source.Page[source.Item]{Total: 1}, nil
	}
	if p, err := s.Search(t.Context(), source.SearchQuery{Query: "x"}); err != nil || p.Total != 1 {
		t.Fatalf("the answer the test gave: %+v, %v", p, err)
	}
}
