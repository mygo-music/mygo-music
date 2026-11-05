package source

import "sync"

// A Registry holds the sources the app has, and which of them it shows.
type Registry struct {
	mu      sync.RWMutex
	sources []Source
	active  string
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
	r.mu.Lock()
	defer r.mu.Unlock()
	id := s.Info().ID
	for i, old := range r.sources {
		if old.Info().ID == id {
			r.sources[i] = s
			return
		}
	}
	r.sources = append(r.sources, s)
	if r.active == "" {
		r.active = id
	}
}

// Remove removes the source with id. If it was shown, the first left is.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, s := range r.sources {
		if s.Info().ID == id {
			r.sources = append(r.sources[:i:i], r.sources[i+1:]...)
			break
		}
	}
	if r.active == id {
		r.active = ""
		if len(r.sources) > 0 {
			r.active = r.sources[0].Info().ID
		}
	}
}

// Sources lists the sources, in the order they were added.
func (r *Registry) Sources() []Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Source(nil), r.sources...)
}

// Get returns the source with id, or nil.
func (r *Registry) Get(id string) Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sources {
		if s.Info().ID == id {
			return s
		}
	}
	return nil
}

// Active returns the source that is shown, or nil with none.
func (r *Registry) Active() Source {
	r.mu.RLock()
	id := r.active
	r.mu.RUnlock()
	return r.Get(id)
}

// SetActive chooses the source to show, and reports whether there is one
// with id.
func (r *Registry) SetActive(id string) bool {
	if r.Get(id) == nil {
		return false
	}
	r.mu.Lock()
	r.active = id
	r.mu.Unlock()
	return true
}
