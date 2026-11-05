package lyrics

import (
	"sort"
	"strings"
	"time"

	"github.com/mygo-music/mygo-music/internal/source"
)

// FromSource makes the lines to show from the lyrics a source gave: its own
// lines, with the times of their words where it gives them, else the LRC text
// it gave, as Build reads it.
func FromSource(l *source.Lyrics) []Line {
	if l == nil {
		return nil
	}
	if len(l.Lines) == 0 {
		return Build(l.LRC, l.TranslationLRC, "")
	}
	ms := func(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
	lines := make([]Line, 0, len(l.Lines))
	for _, s := range l.Lines {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		line := Line{At: ms(s.StartMS), End: ms(s.EndMS), Text: s.Text, Translation: s.Translation}
		for _, w := range s.Words {
			line.Words = append(line.Words, Word{Text: w.Text, Start: ms(w.StartMS), End: ms(w.EndMS)})
		}
		line.Timed = len(line.Words) > 0
		lines = append(lines, line)
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].At < lines[j].At })
	for i := range lines {
		line := &lines[i]
		if line.Timed {
			if line.End <= line.At {
				line.End = line.Words[len(line.Words)-1].End
			}
			continue
		}
		// A line with words of no times of their own: as Build ends it where
		// the next begins, within six seconds, and shares the time.
		if line.End <= line.At {
			line.End = line.At + 6*time.Second
			if i+1 < len(lines) {
				line.End = min(line.End, lines[i+1].At)
			}
		}
		line.Words = SpreadWords(line.Text, line.At, line.End)
	}
	return lines
}
