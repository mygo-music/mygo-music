package netease

import (
	"github.com/mygo-music/mygo-music/internal/lyrics"
	"github.com/mygo-music/mygo-music/internal/source"
)

// wordTimed makes the lines of lyrics from NetEase's word-timed ones (YRC),
// which the app has no parser for besides this one: with the LRC lines and
// their translation, as lyrics.Build merges them. Only lines whose words
// are timed carry them.
func wordTimed(lrc, translation, yrc string) []source.LyricLine {
	lines := lyrics.Build(lrc, translation, yrc)
	out := make([]source.LyricLine, len(lines))
	for i, l := range lines {
		out[i] = source.LyricLine{StartMS: l.At.Milliseconds(), EndMS: l.End.Milliseconds(), Text: l.Text, Translation: l.Translation}
		if !l.Timed {
			continue
		}
		for _, w := range l.Words {
			out[i].Words = append(out[i].Words, source.LyricWord{StartMS: w.Start.Milliseconds(), EndMS: w.End.Milliseconds(), Text: w.Text})
		}
	}
	return out
}
