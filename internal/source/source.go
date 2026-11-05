package source

import (
	"context"
	"encoding/json"
	"slices"
)

// Capabilities a source declares, and what each lets the app show; the
// names are the SDK specification's (docs/02-manifest.md).
const (
	CapSearch   = "search"
	CapStream   = "stream"
	CapAlbum    = "album"
	CapArtist   = "artist"
	CapPlaylist = "playlist"
	CapUser     = "user"
	CapLyrics   = "lyrics"
	CapAccount  = "account"
	CapLibrary  = "library"
	CapFeed     = "feed"
	CapRadio    = "radio"
	CapComments = "comments"
	CapPodcast  = "podcast"
)

// Features, the optional methods within a capability.
const (
	FeatSuggest           = "search.suggest"
	FeatLike              = "like"
	FeatHistory           = "history"
	FeatPlaylistEdit      = "playlist.edit"
	FeatPlaylistSubscribe = "playlist.subscribe"
	FeatFollow            = "follow"
	FeatPodcastSubscribe  = "podcast.subscribe"
	FeatRadioSimilar      = "radio.similar"
	FeatRadioDislike      = "radio.dislike"
	FeatCommentsLike      = "comments.like"
)

// Info describes a source: its manifest, as far as the app reads it.
type Info struct {
	ID           string
	Name         string
	Capabilities []string
	Features     []string
	// SearchTypes are the kinds of entity a search looks for.
	SearchTypes []string
	// Qualities are the stream qualities, from the lowest.
	Qualities []string
}

// Has reports whether the source declares the capability.
func (i *Info) Has(capability string) bool { return slices.Contains(i.Capabilities, capability) }

// HasFeature reports whether the source declares the feature.
func (i *Info) HasFeature(feature string) bool { return slices.Contains(i.Features, feature) }

// Searches reports whether the source searches for the kind of entity.
func (i *Info) Searches(kind string) bool { return slices.Contains(i.SearchTypes, kind) }

// A Source is a music source. What else it can do, it says by implementing
// the interfaces below, which Info's capabilities and features name; ask
// with As.
type Source interface {
	Info() *Info
}

// As returns the source as the interface T, if it implements it.
func As[T any](s Source) (T, bool) {
	t, ok := s.(T)
	return t, ok
}

// Every method takes a context, which a source stops at when it ends, where
// it can: the built-in one does not yet.

// SearchQuery is what a search asks for: one kind of entity, a page of it.
type SearchQuery struct {
	Query  string
	Type   string // one of Info.SearchTypes
	Cursor string
	Limit  int
}

// Page is a page of a list; Next is empty at its end.
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next,omitempty"`
	Total int    `json:"total,omitempty"`
}

// A Searcher searches.
type Searcher interface {
	Search(ctx context.Context, q SearchQuery) (*Page[Item], error)
}

// SuggestGroup is a kind of entity a search may be after, as it is typed.
type SuggestGroup struct {
	Type  string `json:"type"`
	Items []Item `json:"items"`
}

// A Suggester suggests what a search may be after (FeatSuggest). The groups
// come in the order to show.
type Suggester interface {
	Suggest(ctx context.Context, query string) ([]SuggestGroup, error)
}

// A Streamer says where a track streams from (CapStream).
type Streamer interface {
	Resolve(ctx context.Context, t *Track, quality string) (*Stream, error)
}

// FirstTab is the content of the first tab of an artist's or a user's page,
// which saves asking for it.
type FirstTab struct {
	Tab   string `json:"tab"`
	Items []Item `json:"items"`
	Next  string `json:"next,omitempty"`
}

// Pages of entities, with what they hold.
type (
	AlbumPage struct {
		Album  Album   `json:"album"`
		Tracks []Track `json:"tracks"`
		Next   string  `json:"next,omitempty"`
	}
	PlaylistPage struct {
		Playlist Playlist `json:"playlist"`
		Tracks   []Track  `json:"tracks"`
		Next     string   `json:"next,omitempty"`
	}
	ArtistPage struct {
		Artist Artist    `json:"artist"`
		Tabs   []Tab     `json:"tabs"`
		First  *FirstTab `json:"first,omitempty"`
	}
	UserPage struct {
		User  User      `json:"user"`
		Tabs  []Tab     `json:"tabs"`
		First *FirstTab `json:"first,omitempty"`
	}
	PodcastPage struct {
		Podcast  Podcast `json:"podcast"`
		Episodes []Track `json:"episodes"`
		Next     string  `json:"next,omitempty"`
	}
)

// EntityRef names an entity, with the Extra its source gave it.
type EntityRef struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Extra json.RawMessage `json:"extra,omitempty"`
}

// Albums, Artists, Playlists and Users give the pages of their entity.
type (
	AlbumSource interface {
		Album(ctx context.Context, id, cursor string) (*AlbumPage, error)
	}
	ArtistSource interface {
		Artist(ctx context.Context, id string) (*ArtistPage, error)
	}
	PlaylistSource interface {
		Playlist(ctx context.Context, id, cursor string) (*PlaylistPage, error)
	}
	UserSource interface {
		User(ctx context.Context, id string) (*UserPage, error)
	}
	// A TabLoader loads a tab of an artist's or a user's page.
	TabLoader interface {
		LoadTab(ctx context.Context, e EntityRef, tab, cursor string) (*Page[Item], error)
	}
)

// A LyricsSource gives a track's lyrics (CapLyrics); ErrNotFound for none.
type LyricsSource interface {
	Lyrics(ctx context.Context, t *Track) (*Lyrics, error)
}

// Steps of a login. Types of step.
const (
	StepQR   = "qr"
	StepForm = "form"
	StepDone = "done"
)

// States of a QR step.
const (
	QRWaiting = "waiting"
	QRScanned = "scanned"
	QRExpired = "expired"
)

// A Field is a field of a form step.
type Field struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"` // text, password, phone, code or select
	Label       string   `json:"label"`
	Placeholder string   `json:"placeholder,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []Option `json:"options,omitempty"`
}

// An Option is a choice of a select field.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// An Action is a button of a form besides its submit.
type Action struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// A Step is where a login is, and what it needs.
type Step struct {
	Type  string `json:"type"`
	Token string `json:"token,omitempty"`

	// A QR step: Content is what the code holds.
	Content   string   `json:"content,omitempty"`
	State     string   `json:"state,omitempty"`
	Hint      string   `json:"hint,omitempty"`
	User      *UserRef `json:"user,omitempty"`
	PollMS    int      `json:"poll_ms,omitempty"`
	ExpiresMS int64    `json:"expires_ms,omitempty"`

	// A form step.
	Title       string   `json:"title,omitempty"`
	Message     string   `json:"message,omitempty"`
	Error       string   `json:"error,omitempty"`
	Fields      []Field  `json:"fields,omitempty"`
	Actions     []Action `json:"actions,omitempty"`
	SubmitLabel string   `json:"submit_label,omitempty"`
}

// A LoginMethod is a way to sign in.
type LoginMethod struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// AuthStatus says who is signed in.
type AuthStatus struct {
	LoggedIn bool     `json:"logged_in"`
	User     *UserRef `json:"user,omitempty"`
}

// An Account signs in and out (CapAccount). The session is the source's to
// keep.
type Account interface {
	AuthStatus(ctx context.Context) (*AuthStatus, error)
	AuthMethods(ctx context.Context) ([]LoginMethod, error)
	AuthBegin(ctx context.Context, method string) (*Step, error)
	AuthPoll(ctx context.Context, token string) (*Step, error)
	AuthSubmit(ctx context.Context, token, action string, values map[string]string) (*Step, error)
	AuthCancel(ctx context.Context, token string) error
	Logout(ctx context.Context) error
}

// LibraryPlaylists are the listener's playlists: those they made, the liked
// songs among them, and those they saved.
type LibraryPlaylists struct {
	Owned []Playlist `json:"owned"`
	Saved []Playlist `json:"saved"`
}

// A Library has the listener's playlists (CapLibrary).
type Library interface {
	Playlists(ctx context.Context) (*LibraryPlaylists, error)
}

// A Liker likes tracks (FeatLike). A source gives LikedIDs or LikedState or
// both.
type Liker interface {
	Like(ctx context.Context, t *Track, liked bool) error
}

// LikedIDs lists the ids of the tracks the listener likes.
type LikedIDs interface {
	LikedIDs(ctx context.Context) ([]string, error)
}

// LikedState says which of tracks the listener likes, by id.
type LikedState interface {
	LikedState(ctx context.Context, tracks []Track) (map[string]bool, error)
}

// Devices a track was played on.
const (
	DevicePhone   = "phone"
	DeviceDesktop = "desktop"
	DeviceWeb     = "web"
)

// A Play is a track the listener played.
type Play struct {
	Track    Track `json:"track"`
	PlayedMS int64 `json:"played_ms"`
	// Device is where it was played: a Device, or empty if unknown.
	Device string `json:"device,omitempty"`
}

// A History lists what the listener played last (FeatHistory).
type History interface {
	Recent(ctx context.Context, limit int) ([]Play, error)
}

// Follows lists the artists the listener follows and follows or unfollows
// one (FeatFollow).
type Follows interface {
	FollowedArtists(ctx context.Context, cursor string) (*Page[Artist], error)
	Follow(ctx context.Context, id string, follow bool) error
}

// Subscriptions lists the podcasts the listener subscribes to and
// subscribes or unsubscribes (FeatPodcastSubscribe).
type Subscriptions interface {
	SubscribedPodcasts(ctx context.Context, cursor string) (*Page[Podcast], error)
	SubscribePodcast(ctx context.Context, id string, subscribe bool) error
}

// A PlaylistSaver saves others' playlists (FeatPlaylistSubscribe).
type PlaylistSaver interface {
	SavePlaylist(ctx context.Context, id string, saved bool) error
}

// A PlaylistEditor edits the listener's playlists (FeatPlaylistEdit).
// AddTracks returns ErrConflict for a track a playlist has.
type PlaylistEditor interface {
	CreatePlaylist(ctx context.Context, title string, private bool) (*Playlist, error)
	RenamePlaylist(ctx context.Context, id, title string) error
	DeletePlaylist(ctx context.Context, id string) error
	AddTracks(ctx context.Context, id string, tracks []Track) error
	RemoveTracks(ctx context.Context, id string, tracks []Track) error
}

// A Feed gives the home page's sections (CapFeed).
type Feed interface {
	Home(ctx context.Context) ([]Section, error)
	Section(ctx context.Context, id, cursor string) (*Section, error)
}

// A RadioMode is a way a radio picks tracks.
type RadioMode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// A Radio picks tracks without end (CapRadio).
type Radio interface {
	RadioModes(ctx context.Context) ([]RadioMode, error)
	RadioNext(ctx context.Context, mode string) ([]Track, error)
}

// A RadioDisliker is told a track isn't liked, and plays no more of it
// (FeatRadioDislike).
type RadioDisliker interface {
	RadioDislike(ctx context.Context, t *Track) error
}

// SimilarQuery asks for tracks like Seed: Origin is the track the session
// began with, and PlaylistID the listener's liked songs it draws from.
type SimilarQuery struct {
	Seed       Track
	Origin     *Track
	PlaylistID string
}

// A RadioSimilar picks tracks like one the listener likes (FeatRadioSimilar).
type RadioSimilar interface {
	RadioSimilar(ctx context.Context, q SimilarQuery) ([]Track, error)
}

// A Commenter lists a track's comments (CapComments).
type Commenter interface {
	Comments(ctx context.Context, thread, cursor string, limit int) (*Comments, error)
}

// A CommentLiker likes comments (FeatCommentsLike).
type CommentLiker interface {
	LikeComment(ctx context.Context, thread, id string, liked bool) error
}

// Podcasts lists podcasts and their episodes (CapPodcast).
type Podcasts interface {
	PodcastHome(ctx context.Context) ([]Section, error)
	PodcastSection(ctx context.Context, id, cursor string) (*Section, error)
	Podcast(ctx context.Context, id, cursor string) (*PodcastPage, error)
}
