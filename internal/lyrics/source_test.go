package lyrics

import (
	"reflect"
	"testing"
	"time"

	"github.com/mygo-music/mygo-music/internal/source"
)

// A source's lines come out as the lines to show: words that have times
// keep them and mark the line timed; a line without words has them spread
// over the time until the next line, within six seconds, as Build does for
// LRC; and a line that says nothing is left out.
func TestFromSourceLines(t *testing.T) {
	got := FromSource(&source.Lyrics{Lines: []source.LyricLine{
		{StartMS: 20000, Text: "后一行"},
		{StartMS: 10000, EndMS: 14000, Text: "前 一", Translation: "first",
			Words: []source.LyricWord{{StartMS: 10000, EndMS: 11000, Text: "前 "}, {StartMS: 11000, EndMS: 12500, Text: "一"}}},
		{StartMS: 15000, Text: "  "},
		{StartMS: 16000, Text: "中间"},
	}})
	if len(got) != 3 {
		t.Fatalf("%d lines: %+v", len(got), got)
	}
	first := got[0]
	if !first.Timed || first.At != 10*time.Second || first.End != 14*time.Second || first.Translation != "first" ||
		len(first.Words) != 2 || first.Words[1].Start != 11*time.Second || first.Words[1].End != 12500*time.Millisecond {
		t.Fatalf("timed line %+v", first)
	}
	mid := got[1]
	// The next line begins at 20 s; six seconds after 16 s would be 22: it ends at 20.
	if mid.Timed || mid.Text != "中间" || mid.End != 20*time.Second {
		t.Fatalf("middle line %+v", mid)
	}
	if len(mid.Words) != 2 || mid.Words[0].Start != mid.At || mid.Words[len(mid.Words)-1].End > mid.End {
		t.Fatalf("spread words %+v", mid.Words)
	}
	last := got[2]
	if last.End != 26*time.Second {
		t.Fatalf("the last line ends %v, want six seconds on", last.End)
	}
}

// Lyrics that are LRC text come out as Build reads them, with their
// translation; none at all come out as none.
func TestFromSourceLRC(t *testing.T) {
	lrc, tr := "[00:01.00]你好\n[00:05.00]世界", "[00:01.00]Hello\n[00:05.00]World"
	if got, want := FromSource(&source.Lyrics{LRC: lrc, TranslationLRC: tr}), Build(lrc, tr, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if FromSource(nil) != nil || len(FromSource(&source.Lyrics{})) != 0 {
		t.Fatal("lines from no lyrics")
	}
}
