package source

import "sync"

// A Registry holds the sources the app has, and which of them it shows.
type Registry struct {
	mu      sync.RWMutex
	entries []entry
	active  string
}

// entry keeps a source with its ID, which the registry reads once: a source
// whose Info() is slow, as a plugin's may be, is never asked under the lock.
type entry struct {
	id string
	s  Source
}

// NewRegistry returns a registry of sources, the first of them shown.
func NewRegistry(sources ...Source) *Registry {
	r := &Registry{}
	for _, s := range sources {
		r.Add(s)
	}
	return r
}

// Add adds a source, or replaces the one with its ID. The first added is
// shown until another is chosen.
func (r *Registry) Add(s Source) {
	id := s.Info().ID
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.entries {
		if e.id == id {
			r.entries[i].s = s
			return
		}
	}
	r.entries = append(r.entries, entry{id, s})
	if r.active == "" {
		r.active = id
	}
}

// Remove removes the source with id. If it was shown, the first left is.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.entries {
		if e.id == id {
			r.entries = append(r.entries[:i:i], r.entries[i+1:]...)
			break
		}
	}
	if r.active == id {
		r.active = ""
		if len(r.entries) > 0 {
			r.active = r.entries[0].id
		}
	}
}

// Sources lists the sources, in the order they were added.
func (r *Registry) Sources() []Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Source, len(r.entries))
	for i, e := range r.entries {
		out[i] = e.s
	}
	return out
}

func (r *Registry) get(id string) Source {
	for _, e := range r.entries {
		if e.id == id {
			return e.s
		}
	}
	return nil
}

// Get returns the source with id, or nil.
func (r *Registry) Get(id string) Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.get(id)
}

// Active returns the source that is shown, or nil with none.
func (r *Registry) Active() Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.get(r.active)
}

// SetActive chooses the source to show, and reports whether there is one
// with id.
func (r *Registry) SetActive(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.get(id) == nil {
		return false
	}
	r.active = id
	return true
}
