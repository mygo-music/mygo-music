package app

import "github.com/mygo-music/mygo-music/internal/source"

// The sources the app has are in its registry, and one of them shows: what
// the pages list, search, play and sign in to.

// active returns the source that shows, or nil with none.
func (a *app) active() source.Source {
	if a.reg == nil {
		return nil
	}
	return a.reg.Active()
}

// capOf returns the shown source as the interface T, if it is one: of the
// capabilities and features a source declares, it implements those.
func capOf[T any](a *app) (T, bool) {
	var zero T
	s := a.active()
	if s == nil {
		return zero, false
	}
	return source.As[T](s)
}

// sourceName is what the shown source is called, as the listener reads it.
func (a *app) sourceName() string {
	if s := a.active(); s != nil {
		return s.Info().Name
	}
	return "音乐来源"
}
