// Package source is what the app knows of a music source: the data it shows
// and the calls it makes, with no mention of any one service. A source is a
// plugin, or until plugins are loaded, the one built in.
//
// The types and their JSON are those of the plugin SDK's specification
// (docs/04-data-model.md), so what a plugin answers decodes into them as it
// is. IDs are strings the app does not read; the Go names follow the app's.
package source

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ArtistRef, AlbumRef and UserRef name an artist, an album or a user where
// another entity mentions them.
type (
	ArtistRef struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name"`
	}
	AlbumRef struct {
		ID    string `json:"id,omitempty"`
		Name  string `json:"name"`
		Cover string `json:"cover,omitempty"`
	}
	UserRef struct {
		ID     string `json:"id,omitempty"`
		Name   string `json:"name"`
		Avatar string `json:"avatar,omitempty"`
	}
)

// Kinds of track.
const (
	KindSong    = "song"
	KindEpisode = "episode"
)

// PodcastRef names the podcast an episode is of.
type PodcastRef struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
}

// Episode is what a track that is a podcast's episode is besides its audio.
type Episode struct {
	ID          string     `json:"id"`
	Podcast     PodcastRef `json:"podcast"`
	Description string     `json:"description,omitempty"`
	PublishedMS int64      `json:"published_ms,omitempty"`
}

// A Track is a song or an episode that plays.
type Track struct {
	// Plugin is the source the track came from. The source sets it, to its ID
	// (the app trusts no other).
	Plugin string `json:"plugin,omitempty"`
	// ID is what Streamer.Resolve takes: a song's, or an episode's audio.
	ID    string `json:"id"`
	Title string `json:"title"`
	// Artists may be none; the JSON leaves them out then, not null.
	Artists []ArtistRef `json:"artists,omitempty"`
	Album   AlbumRef    `json:"album,omitzero"`
	// DurationMS is the length in milliseconds.
	DurationMS int64  `json:"duration_ms,omitempty"`
	Cover      string `json:"cover,omitempty"`
	// Playable is nil for a track that plays, and points to false for one the
	// listener can't play whole: a Track made in Go plays, as one that says
	// nothing of it in JSON does. Ask with CanPlay.
	Playable         *bool    `json:"playable,omitempty"`
	UnplayableReason string   `json:"unplayable_reason,omitempty"`
	Aliases          []string `json:"aliases,omitempty"`
	// Reason says why a recommendation was picked.
	Reason string `json:"reason,omitempty"`
	Kind   string `json:"kind,omitempty"`
	// Episode is set when Kind is KindEpisode.
	Episode *Episode `json:"episode,omitempty"`
	// CommentThread names the track's comments; with none, it has none.
	CommentThread string `json:"comment_thread,omitempty"`
	// URL is the track's page on the source's website, to copy as a link.
	URL string `json:"url,omitempty"`
	// Extra is the source's own, kept and given back to it as it is.
	Extra json.RawMessage `json:"extra,omitempty"`
}

// CanPlay reports whether the track plays whole.
func (t *Track) CanPlay() bool { return t.Playable == nil || *t.Playable }

// SetUnplayable marks the track as one that can't play, and says why.
func (t *Track) SetUnplayable(reason string) {
	no := false
	t.Playable, t.UnplayableReason = &no, reason
}

// ArtistNames joins the track's artists as the credits read.
func (t *Track) ArtistNames() string { return joinArtists(t.Artists) }

// Length is how long the track plays.
func (t *Track) Length() time.Duration { return time.Duration(t.DurationMS) * time.Millisecond }

// IsEpisode reports whether the track is a podcast's episode.
func (t *Track) IsEpisode() bool { return t.Kind == KindEpisode }

// CoverURL is the track's cover, or its album's.
func (t *Track) CoverURL() string {
	if t.Cover != "" {
		return t.Cover
	}
	return t.Album.Cover
}

// Key names the track among those of all sources, as a cache keeps it.
func (t *Track) Key() string { return t.Plugin + ":" + t.ID }

func joinArtists(artists []ArtistRef) string {
	names := make([]string, len(artists))
	for i, a := range artists {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

// An Album is a record, as its page shows it.
type Album struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Artists     []ArtistRef     `json:"artists,omitempty"`
	Cover       string          `json:"cover,omitempty"`
	ReleasedMS  int64           `json:"released_ms,omitempty"`
	TrackCount  int             `json:"track_count,omitempty"`
	Description string          `json:"description,omitempty"`
	Subtitle    string          `json:"subtitle,omitempty"`
	Extra       json.RawMessage `json:"extra,omitempty"`
}

// ArtistNames joins the album's artists as its credits read.
func (a *Album) ArtistNames() string { return joinArtists(a.Artists) }

// An Artist is who makes music, as their page shows them.
type Artist struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Avatar      string          `json:"avatar,omitempty"`
	Aliases     []string        `json:"aliases,omitempty"`
	Description string          `json:"description,omitempty"`
	Followed    bool            `json:"followed,omitempty"`
	TrackCount  int             `json:"track_count,omitempty"`
	AlbumCount  int             `json:"album_count,omitempty"`
	Extra       json.RawMessage `json:"extra,omitempty"`
}

// Roles of a playlist.
const (
	RoleNormal = "normal"
	// RoleLiked is the listener's liked songs.
	RoleLiked = "liked"
)

// PreviewLine is a song at the top of a chart: its title and its artists.
type PreviewLine struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

// A Playlist is a list of tracks somebody made, or a chart.
type Playlist struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Cover       string          `json:"cover,omitempty"`
	Description string          `json:"description,omitempty"`
	TrackCount  int             `json:"track_count,omitempty"`
	PlayCount   int64           `json:"play_count,omitempty"`
	Creator     UserRef         `json:"creator,omitzero"`
	UpdateText  string          `json:"update_text,omitempty"`
	Role        string          `json:"role,omitempty"`
	Private     bool            `json:"private,omitempty"`
	Owned       bool            `json:"owned,omitempty"`
	Subscribed  bool            `json:"subscribed,omitempty"`
	Preview     []PreviewLine   `json:"preview,omitempty"`
	Extra       json.RawMessage `json:"extra,omitempty"`
}

// IsLiked reports whether the playlist is the listener's liked songs.
func (p *Playlist) IsLiked() bool { return p.Role == RoleLiked }

// A User is someone on the source, as their page shows them.
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar,omitempty"`
	Description string `json:"description,omitempty"`
	Followed    bool   `json:"followed,omitempty"`
	// FollowerCount and FollowingCount are 0 where unknown.
	FollowerCount  int64           `json:"follower_count,omitempty"`
	FollowingCount int64           `json:"following_count,omitempty"`
	Extra          json.RawMessage `json:"extra,omitempty"`
}

// A Podcast is a channel of episodes.
type Podcast struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Cover           string          `json:"cover,omitempty"`
	Description     string          `json:"description,omitempty"`
	Category        string          `json:"category,omitempty"`
	Host            UserRef         `json:"host,omitzero"`
	EpisodeCount    int             `json:"episode_count,omitempty"`
	SubscriberCount int64           `json:"subscriber_count,omitempty"`
	PlayCount       int64           `json:"play_count,omitempty"`
	Subscribed      bool            `json:"subscribed,omitempty"`
	Tagline         string          `json:"tagline,omitempty"`
	LatestEpisode   string          `json:"latest_episode,omitempty"`
	Extra           json.RawMessage `json:"extra,omitempty"`
}

// A Post is something an artist or a user posted to their feed: words,
// pictures, and maybe what it shares.
type Post struct {
	ID                  string   `json:"id"`
	Author              UserRef  `json:"author,omitzero"`
	TimeMS              int64    `json:"time_ms,omitempty"`
	Text                string   `json:"text,omitempty"`
	Images              []string `json:"images,omitempty"`
	Attachment          *Item    `json:"attachment,omitempty"`
	Forwarded           string   `json:"forwarded,omitempty"`
	LikeCount           int      `json:"like_count,omitempty"`
	CommentCount        int      `json:"comment_count,omitempty"`
	ShareCount          int      `json:"share_count,omitempty"`
	HasUnsupportedMedia bool     `json:"has_unsupported_media,omitempty"`
}

// Kinds of entity, as an Item and a Tab name them.
const (
	TypeTrack    = "track"
	TypeAlbum    = "album"
	TypeArtist   = "artist"
	TypePlaylist = "playlist"
	TypeUser     = "user"
	TypePodcast  = "podcast"
	TypePost     = "post"
)

// An Item is an entity of any kind, as a section or a search lists them:
// exactly one of the fields is set, the one Type names.
type Item struct {
	Type     string
	Track    *Track
	Album    *Album
	Artist   *Artist
	Playlist *Playlist
	User     *User
	Podcast  *Podcast
	Post     *Post
}

// UnmarshalJSON reads an item by its "type".
func (it *Item) UnmarshalJSON(b []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return err
	}
	*it = Item{Type: head.Type}
	var into any
	switch head.Type {
	case TypeTrack:
		it.Track = new(Track)
		into = it.Track
	case TypeAlbum:
		it.Album = new(Album)
		into = it.Album
	case TypeArtist:
		it.Artist = new(Artist)
		into = it.Artist
	case TypePlaylist:
		it.Playlist = new(Playlist)
		into = it.Playlist
	case TypeUser:
		it.User = new(User)
		into = it.User
	case TypePodcast:
		it.Podcast = new(Podcast)
		into = it.Podcast
	case TypePost:
		it.Post = new(Post)
		into = it.Post
	default:
		return nil // a kind this version doesn't know: left empty, to skip
	}
	return json.Unmarshal(b, into)
}

// MarshalJSON writes an item with its "type".
func (it Item) MarshalJSON() ([]byte, error) {
	var v any
	switch it.Type {
	case TypeTrack:
		v = it.Track
	case TypeAlbum:
		v = it.Album
	case TypeArtist:
		v = it.Artist
	case TypePlaylist:
		v = it.Playlist
	case TypeUser:
		v = it.User
	case TypePodcast:
		v = it.Podcast
	case TypePost:
		v = it.Post
	default:
		return []byte("null"), nil
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) < 2 || b[0] != '{' {
		return b, err
	}
	typ, _ := json.Marshal(it.Type)
	head := append([]byte(`{"type":`), typ...)
	if len(b) > 2 {
		head = append(head, ',')
	}
	return append(head, b[1:]...), nil
}

// Empty reports whether the item is of a kind this version doesn't know.
func (it Item) Empty() bool {
	return it.Track == nil && it.Album == nil && it.Artist == nil && it.Playlist == nil &&
		it.User == nil && it.Podcast == nil && it.Post == nil
}

// Section styles, which hint at how a section shows.
const (
	StyleList  = "list"
	StyleGrid  = "grid"
	StyleHero  = "hero"
	StyleChart = "chart"
)

// A Section is a block of a page: the home page's daily songs, its charts.
type Section struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Style    string `json:"style,omitempty"`
	// PlayAll shows a button to play the section's tracks.
	PlayAll bool   `json:"play_all,omitempty"`
	Items   []Item `json:"items"`
	Next    string `json:"next,omitempty"`
	// More says the section has a page of its own, which Feed.Section loads.
	More bool `json:"more,omitempty"`
	// ExpiresMS is when the source changes what the section holds, as the
	// daily songs at 6:00: the app loads it again then. 0 for never.
	ExpiresMS int64 `json:"expires_ms,omitempty"`
}

// Types of tab: what a tab lists, in the plural the specification writes.
const (
	TabTracks    = "tracks"
	TabAlbums    = "albums"
	TabArtists   = "artists"
	TabPlaylists = "playlists"
	TabPodcasts  = "podcasts"
	TabUsers     = "users"
	TabPosts     = "posts"
)

// A Tab is a part of an artist's or a user's page, the source's to name and
// to order.
type Tab struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Type is what the tab lists: one of the Tab types.
	Type string `json:"type"`
}

// Formats of a stream, the ones the player decodes.
const (
	FormatMP3  = "mp3"
	FormatFLAC = "flac"
)

// Qualities of a stream, from the lowest.
var Qualities = []string{"standard", "higher", "exhigh", "lossless", "hires"}

// Lossless reports whether quality is lossless.
func Lossless(quality string) bool { return quality == "lossless" || quality == "hires" }

// A Stream is where a track streams from, and what it is.
type Stream struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Format  string            `json:"format"`
	// Quality is the one served, which may be below the one asked for.
	Quality   string `json:"quality"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	// GainDB is how many dB to change the track by to sound as loud as
	// others, and Peak its loudest sample, 1 at full scale; 0 if unknown.
	GainDB    float64 `json:"gain_db,omitempty"`
	Peak      float64 `json:"peak,omitempty"`
	ExpiresMS int64   `json:"expires_ms,omitempty"`
}

// Lyrics are a track's, as lines or as LRC text, the lines first.
type Lyrics struct {
	Lines          []LyricLine `json:"lines,omitempty"`
	LRC            string      `json:"lrc,omitempty"`
	TranslationLRC string      `json:"translation_lrc,omitempty"`
	Instrumental   bool        `json:"instrumental,omitempty"`
}

// A LyricLine is a line of timed lyrics; its Words, when each is sung.
type LyricLine struct {
	StartMS     int64       `json:"start_ms"`
	EndMS       int64       `json:"end_ms,omitempty"`
	Text        string      `json:"text"`
	Translation string      `json:"translation,omitempty"`
	Words       []LyricWord `json:"words,omitempty"`
}

// A LyricWord is a word of a line, with when it is sung.
type LyricWord struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Text    string `json:"text"`
}

// A Comment is what a listener wrote about a track.
type Comment struct {
	ID        string  `json:"id"`
	User      UserRef `json:"user"`
	Text      string  `json:"text"`
	TimeMS    int64   `json:"time_ms,omitempty"`
	LikeCount int     `json:"like_count,omitempty"`
	Liked     bool    `json:"liked,omitempty"`
	Location  string  `json:"location,omitempty"`
	ReplyTo   *Reply  `json:"reply_to,omitempty"`
}

// Reply is the comment a comment answers.
type Reply struct {
	UserName string `json:"user_name"`
	Text     string `json:"text"`
}

// Comments are a page of a track's comments; the hot ones come first, on
// the first page only.
type Comments struct {
	Hot   []Comment `json:"hot,omitempty"`
	Items []Comment `json:"items"`
	Total int       `json:"total,omitempty"`
	Next  string    `json:"next,omitempty"`
}

// Sized gives an image URL the size it is shown at: a "{size}" in it is
// replaced by px, the edge of a square.
func Sized(url string, px int) string {
	if !strings.Contains(url, "{size}") {
		return url
	}
	return strings.ReplaceAll(url, "{size}", strconv.Itoa(px))
}
