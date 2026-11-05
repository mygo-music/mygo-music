package app

import (
	"context"
	"errors"

	"github.com/mygo-music/mygo-music/internal/source"
)

// errOffline is what a fake source answers for a call the test gave it no
// answer to: the network is down for it.
var errOffline = errors.New("offline: the test gave the fake source no answer for this")

// fakeSource is the source the tests of the app use, with a function for each
// call a test may answer; a call it has no function for fails with
// errOffline. Each test app has its own, so a test needs to restore nothing.
// It declares every capability, so that what it hasn't an answer for fails
// where the app asks, not where it decides to ask.
type fakeSource struct {
	search          func(source.SearchQuery) (*source.Page[source.Item], error)
	suggest         func(query string) ([]source.SuggestGroup, error)
	resolve         func(t *source.Track, quality string) (*source.Stream, error)
	album           func(id, cursor string) (*source.AlbumPage, error)
	artist          func(id string) (*source.ArtistPage, error)
	playlist        func(id, cursor string) (*source.PlaylistPage, error)
	user            func(id string) (*source.UserPage, error)
	loadTab         func(e source.EntityRef, tab, cursor string) (*source.Page[source.Item], error)
	lyrics          func(t *source.Track) (*source.Lyrics, error)
	authStatus      func() (*source.AuthStatus, error)
	authMethods     func() ([]source.LoginMethod, error)
	authBegin       func(method string) (*source.Step, error)
	authPoll        func(token string) (*source.Step, error)
	authSubmit      func(token, action string, values map[string]string) (*source.Step, error)
	authCancel      func(token string) error
	logout          func() error
	playlists       func() (*source.LibraryPlaylists, error)
	like            func(t *source.Track, liked bool) error
	likedIDs        func() ([]string, error)
	likedState      func(tracks []source.Track) (map[string]bool, error)
	recent          func(limit int) ([]source.Play, error)
	followedArtists func(cursor string) (*source.Page[source.Artist], error)
	follow          func(id string, follow bool) error
	subscribed      func(cursor string) (*source.Page[source.Podcast], error)
	subscribe       func(id string, subscribe bool) error
	savePlaylist    func(id string, saved bool) error
	createPlaylist  func(title string, private bool) (*source.Playlist, error)
	renamePlaylist  func(id, title string) error
	deletePlaylist  func(id string) error
	addTracks       func(id string, tracks []source.Track) error
	removeTracks    func(id string, tracks []source.Track) error
	home            func() ([]source.Section, error)
	section         func(id, cursor string) (*source.Section, error)
	radioModes      func() ([]source.RadioMode, error)
	radioNext       func(mode string) ([]source.Track, error)
	radioDislike    func(t *source.Track) error
	radioSimilar    func(q source.SimilarQuery) ([]source.Track, error)
	comments        func(thread, cursor string, limit int) (*source.Comments, error)
	likeComment     func(thread, id string, liked bool) error
	podcastHome     func() ([]source.Section, error)
	podcastSection  func(id, cursor string) (*source.Section, error)
	podcast         func(id, cursor string) (*source.PodcastPage, error)
}

var _ interface {
	source.Source
	source.Searcher
	source.Suggester
	source.Streamer
	source.AlbumSource
	source.ArtistSource
	source.PlaylistSource
	source.UserSource
	source.TabLoader
	source.LyricsSource
	source.Account
	source.Library
	source.Liker
	source.LikedIDs
	source.LikedState
	source.History
	source.Follows
	source.Subscriptions
	source.PlaylistSaver
	source.PlaylistEditor
	source.Feed
	source.Radio
	source.RadioDisliker
	source.RadioSimilar
	source.Commenter
	source.CommentLiker
	source.Podcasts
} = (*fakeSource)(nil)

// fakeInfo is the fake's manifest: all there is to declare.
var fakeInfo = source.Info{
	ID:   "fake",
	Name: "测试来源",
	Capabilities: []string{
		source.CapSearch, source.CapStream, source.CapAlbum, source.CapArtist, source.CapPlaylist,
		source.CapUser, source.CapLyrics, source.CapAccount, source.CapLibrary, source.CapFeed,
		source.CapRadio, source.CapComments, source.CapPodcast,
	},
	Features: []string{
		source.FeatSuggest, source.FeatLike, source.FeatHistory, source.FeatPlaylistEdit,
		source.FeatPlaylistSubscribe, source.FeatFollow, source.FeatPodcastSubscribe,
		source.FeatRadioSimilar, source.FeatRadioDislike, source.FeatCommentsLike,
	},
	SearchTypes: []string{
		source.TypeTrack, source.TypeArtist, source.TypeAlbum, source.TypePlaylist, source.TypeUser, source.TypePodcast,
	},
	Qualities: source.Qualities,
}

func (f *fakeSource) Info() *source.Info { return &fakeInfo }

// fakeOf returns the fake source a test app has, to give it answers.
func fakeOf(a *app) *fakeSource { return a.reg.Active().(*fakeSource) }

func (f *fakeSource) Search(_ context.Context, q source.SearchQuery) (*source.Page[source.Item], error) {
	if f.search == nil {
		return nil, errOffline
	}
	return f.search(q)
}

func (f *fakeSource) Suggest(_ context.Context, query string) ([]source.SuggestGroup, error) {
	if f.suggest == nil {
		return nil, errOffline
	}
	return f.suggest(query)
}

func (f *fakeSource) Resolve(_ context.Context, t *source.Track, quality string) (*source.Stream, error) {
	if f.resolve == nil {
		return nil, errOffline
	}
	return f.resolve(t, quality)
}

func (f *fakeSource) Album(_ context.Context, id, cursor string) (*source.AlbumPage, error) {
	if f.album == nil {
		return nil, errOffline
	}
	return f.album(id, cursor)
}

func (f *fakeSource) Artist(_ context.Context, id string) (*source.ArtistPage, error) {
	if f.artist == nil {
		return nil, errOffline
	}
	return f.artist(id)
}

func (f *fakeSource) Playlist(_ context.Context, id, cursor string) (*source.PlaylistPage, error) {
	if f.playlist == nil {
		return nil, errOffline
	}
	return f.playlist(id, cursor)
}

func (f *fakeSource) User(_ context.Context, id string) (*source.UserPage, error) {
	if f.user == nil {
		return nil, errOffline
	}
	return f.user(id)
}

func (f *fakeSource) LoadTab(_ context.Context, e source.EntityRef, tab, cursor string) (*source.Page[source.Item], error) {
	if f.loadTab == nil {
		return nil, errOffline
	}
	return f.loadTab(e, tab, cursor)
}

func (f *fakeSource) Lyrics(_ context.Context, t *source.Track) (*source.Lyrics, error) {
	if f.lyrics == nil {
		return nil, errOffline
	}
	return f.lyrics(t)
}

func (f *fakeSource) AuthStatus(context.Context) (*source.AuthStatus, error) {
	if f.authStatus == nil {
		return nil, errOffline
	}
	return f.authStatus()
}

func (f *fakeSource) AuthMethods(context.Context) ([]source.LoginMethod, error) {
	if f.authMethods == nil {
		return nil, errOffline
	}
	return f.authMethods()
}

func (f *fakeSource) AuthBegin(_ context.Context, method string) (*source.Step, error) {
	if f.authBegin == nil {
		return nil, errOffline
	}
	return f.authBegin(method)
}

func (f *fakeSource) AuthPoll(_ context.Context, token string) (*source.Step, error) {
	if f.authPoll == nil {
		return nil, errOffline
	}
	return f.authPoll(token)
}

func (f *fakeSource) AuthSubmit(_ context.Context, token, action string, values map[string]string) (*source.Step, error) {
	if f.authSubmit == nil {
		return nil, errOffline
	}
	return f.authSubmit(token, action, values)
}

func (f *fakeSource) AuthCancel(_ context.Context, token string) error {
	if f.authCancel == nil {
		return errOffline
	}
	return f.authCancel(token)
}

func (f *fakeSource) Logout(context.Context) error {
	if f.logout == nil {
		return errOffline
	}
	return f.logout()
}

func (f *fakeSource) Playlists(context.Context) (*source.LibraryPlaylists, error) {
	if f.playlists == nil {
		return nil, errOffline
	}
	return f.playlists()
}

func (f *fakeSource) Like(_ context.Context, t *source.Track, liked bool) error {
	if f.like == nil {
		return errOffline
	}
	return f.like(t, liked)
}

func (f *fakeSource) LikedIDs(context.Context) ([]string, error) {
	if f.likedIDs == nil {
		return nil, errOffline
	}
	return f.likedIDs()
}

func (f *fakeSource) LikedState(_ context.Context, tracks []source.Track) (map[string]bool, error) {
	if f.likedState == nil {
		return nil, errOffline
	}
	return f.likedState(tracks)
}

func (f *fakeSource) Recent(_ context.Context, limit int) ([]source.Play, error) {
	if f.recent == nil {
		return nil, errOffline
	}
	return f.recent(limit)
}

func (f *fakeSource) FollowedArtists(_ context.Context, cursor string) (*source.Page[source.Artist], error) {
	if f.followedArtists == nil {
		return nil, errOffline
	}
	return f.followedArtists(cursor)
}

func (f *fakeSource) Follow(_ context.Context, id string, follow bool) error {
	if f.follow == nil {
		return errOffline
	}
	return f.follow(id, follow)
}

func (f *fakeSource) SubscribedPodcasts(_ context.Context, cursor string) (*source.Page[source.Podcast], error) {
	if f.subscribed == nil {
		return nil, errOffline
	}
	return f.subscribed(cursor)
}

func (f *fakeSource) SubscribePodcast(_ context.Context, id string, subscribe bool) error {
	if f.subscribe == nil {
		return errOffline
	}
	return f.subscribe(id, subscribe)
}

func (f *fakeSource) SavePlaylist(_ context.Context, id string, saved bool) error {
	if f.savePlaylist == nil {
		return errOffline
	}
	return f.savePlaylist(id, saved)
}

func (f *fakeSource) CreatePlaylist(_ context.Context, title string, private bool) (*source.Playlist, error) {
	if f.createPlaylist == nil {
		return nil, errOffline
	}
	return f.createPlaylist(title, private)
}

func (f *fakeSource) RenamePlaylist(_ context.Context, id, title string) error {
	if f.renamePlaylist == nil {
		return errOffline
	}
	return f.renamePlaylist(id, title)
}

func (f *fakeSource) DeletePlaylist(_ context.Context, id string) error {
	if f.deletePlaylist == nil {
		return errOffline
	}
	return f.deletePlaylist(id)
}

func (f *fakeSource) AddTracks(_ context.Context, id string, tracks []source.Track) error {
	if f.addTracks == nil {
		return errOffline
	}
	return f.addTracks(id, tracks)
}

func (f *fakeSource) RemoveTracks(_ context.Context, id string, tracks []source.Track) error {
	if f.removeTracks == nil {
		return errOffline
	}
	return f.removeTracks(id, tracks)
}

func (f *fakeSource) Home(context.Context) ([]source.Section, error) {
	if f.home == nil {
		return nil, errOffline
	}
	return f.home()
}

func (f *fakeSource) Section(_ context.Context, id, cursor string) (*source.Section, error) {
	if f.section == nil {
		return nil, errOffline
	}
	return f.section(id, cursor)
}

func (f *fakeSource) RadioModes(context.Context) ([]source.RadioMode, error) {
	if f.radioModes == nil {
		return nil, errOffline
	}
	return f.radioModes()
}

func (f *fakeSource) RadioNext(_ context.Context, mode string) ([]source.Track, error) {
	if f.radioNext == nil {
		return nil, errOffline
	}
	return f.radioNext(mode)
}

func (f *fakeSource) RadioDislike(_ context.Context, t *source.Track) error {
	if f.radioDislike == nil {
		return errOffline
	}
	return f.radioDislike(t)
}

func (f *fakeSource) RadioSimilar(_ context.Context, q source.SimilarQuery) ([]source.Track, error) {
	if f.radioSimilar == nil {
		return nil, errOffline
	}
	return f.radioSimilar(q)
}

func (f *fakeSource) Comments(_ context.Context, thread, cursor string, limit int) (*source.Comments, error) {
	if f.comments == nil {
		return nil, errOffline
	}
	return f.comments(thread, cursor, limit)
}

func (f *fakeSource) LikeComment(_ context.Context, thread, id string, liked bool) error {
	if f.likeComment == nil {
		return errOffline
	}
	return f.likeComment(thread, id, liked)
}

func (f *fakeSource) PodcastHome(context.Context) ([]source.Section, error) {
	if f.podcastHome == nil {
		return nil, errOffline
	}
	return f.podcastHome()
}

func (f *fakeSource) PodcastSection(_ context.Context, id, cursor string) (*source.Section, error) {
	if f.podcastSection == nil {
		return nil, errOffline
	}
	return f.podcastSection(id, cursor)
}

func (f *fakeSource) Podcast(_ context.Context, id, cursor string) (*source.PodcastPage, error) {
	if f.podcast == nil {
		return nil, errOffline
	}
	return f.podcast(id, cursor)
}
