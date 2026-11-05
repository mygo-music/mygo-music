// Package netease is NetEase Cloud Music as a source: it answers the calls
// of the source package with what the client in internal/netease gets from
// the service. It is the one source built into the app, and the code that
// becomes the official plugin.
package netease

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"

	api "github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// PluginID is the source's ID, which the tracks it gives carry.
const PluginID = "netease"

// Source is NetEase Cloud Music.
type Source struct {
	mu  sync.Mutex
	uid int64 // who is signed in, once known; 0 for nobody
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
} = (*Source)(nil)

// New returns the source, with the session the last run kept, if any.
func New() *Source {
	api.RestoreSession()
	return &Source{}
}

var info = source.Info{
	ID:   PluginID,
	Name: "网易云音乐",
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
		source.TypeTrack, source.TypeAlbum, source.TypeArtist, source.TypePlaylist, source.TypeUser, source.TypePodcast,
	},
	Qualities: api.Levels,
}

func (s *Source) Info() *source.Info { return &info }

// Page sizes the source asks the service for.
const (
	latestCount  = 100
	albumsPage   = 30
	eventsPage   = 20
	podcastsPage = 30
)

// me returns who is signed in, asking the service the first time. Nobody
// is ErrUnauthenticated.
func (s *Source) me() (int64, error) {
	s.mu.Lock()
	uid := s.uid
	s.mu.Unlock()
	if uid != 0 {
		return uid, nil
	}
	p, err := api.Account()
	if err != nil {
		return 0, source.Wrap(source.CodeNetwork, err)
	}
	if p == nil {
		return 0, source.ErrUnauthenticated
	}
	s.mu.Lock()
	s.uid = p.UserID
	s.mu.Unlock()
	return p.UserID, nil
}

func (s *Source) forget() {
	s.mu.Lock()
	s.uid = 0
	s.mu.Unlock()
}

// offsetOf reads where a page starts: an offset, 0 for the first.
func offsetOf(c string) int {
	n, _ := strconv.Atoi(c)
	return max(n, 0)
}

// next is the cursor of the page after one of n items from offset, when
// more are to come.
func next(offset, n int, more bool) string {
	if !more || n == 0 {
		return ""
	}
	return strconv.Itoa(offset + n)
}

func failed(err error) error { return source.Wrap(source.CodeNetwork, err) }

var kinds = map[string]api.SearchType{
	source.TypeTrack:    api.SearchSongs,
	source.TypeAlbum:    api.SearchAlbums,
	source.TypeArtist:   api.SearchArtists,
	source.TypePlaylist: api.SearchPlaylists,
	source.TypeUser:     api.SearchUsers,
	source.TypePodcast:  api.SearchPodcasts,
}

func (s *Source) Search(ctx context.Context, q source.SearchQuery) (*source.Page[source.Item], error) {
	kind, ok := kinds[q.Type]
	if !ok {
		return nil, source.Fail(source.CodeInvalidArgument, "不能搜索"+q.Type)
	}
	offset := offsetOf(q.Cursor)
	res, err := api.SearchPage(q.Query, kind, max(q.Limit, 1), offset)
	if err != nil {
		return nil, failed(err)
	}
	var items []source.Item
	switch kind {
	case api.SearchSongs:
		items = trackItems(res.Songs)
	case api.SearchAlbums:
		items = albumItems(res.Albums)
	case api.SearchArtists:
		items = artistItems(res.Artists)
	case api.SearchPlaylists:
		items = playlistItems(res.Playlists)
	case api.SearchUsers:
		items = userItems(res.Users)
	case api.SearchPodcasts:
		items = podcastItems(res.Podcasts)
	}
	return &source.Page[source.Item]{Items: items, Next: next(offset, len(items), offset+len(items) < res.Total), Total: res.Total}, nil
}

func (s *Source) Suggest(ctx context.Context, query string) ([]source.SuggestGroup, error) {
	res, err := api.SearchSuggest(query)
	if err != nil {
		return nil, failed(err)
	}
	var groups []source.SuggestGroup
	for _, kind := range res.Order {
		switch kind {
		case "songs":
			groups = append(groups, source.SuggestGroup{Type: source.TypeTrack, Items: trackItems(res.Songs)})
		case "artists":
			ars := make([]api.ArtistProfile, len(res.Artists))
			for i, a := range res.Artists {
				ars[i] = api.ArtistProfile{Artist: a}
			}
			groups = append(groups, source.SuggestGroup{Type: source.TypeArtist, Items: artistItems(ars)})
		case "albums":
			groups = append(groups, source.SuggestGroup{Type: source.TypeAlbum, Items: albumItems(res.Albums)})
		case "playlists":
			groups = append(groups, source.SuggestGroup{Type: source.TypePlaylist, Items: playlistItems(res.Playlists)})
		}
	}
	return groups, nil
}

func (s *Source) Resolve(ctx context.Context, t *source.Track, quality string) (*source.Stream, error) {
	n, err := number(t.ID)
	if err != nil {
		return nil, err
	}
	st, err := api.SongURL(n, quality)
	switch {
	case errors.Is(err, api.ErrUnavailable):
		return nil, source.Wrap(source.CodeUnavailable, err)
	case err != nil:
		return nil, failed(err)
	}
	format := st.Type
	if format == "" {
		format = source.FormatMP3
		if source.Lossless(st.Level) {
			format = source.FormatFLAC
		}
	}
	return &source.Stream{
		URL:       st.URL,
		Headers:   map[string]string{"User-Agent": api.UserAgent},
		Format:    format,
		Quality:   st.Level,
		SizeBytes: st.Size,
		GainDB:    st.Gain,
		Peak:      st.Peak,
	}, nil
}

func (s *Source) Album(ctx context.Context, albumID, cursor string) (*source.AlbumPage, error) {
	n, err := number(albumID)
	if err != nil {
		return nil, err
	}
	al, err := api.AlbumDetail(n)
	if err != nil {
		return nil, failed(err)
	}
	return &source.AlbumPage{Album: album(*al), Tracks: tracks(al.Songs)}, nil
}

// The tabs of an artist's page.
const (
	tabHot    = "hot"
	tabLatest = "latest"
	tabAlbums = "albums"
	tabEvents = "events"
)

func (s *Source) Artist(ctx context.Context, artistID string) (*source.ArtistPage, error) {
	n, err := number(artistID)
	if err != nil {
		return nil, err
	}
	ar, err := api.ArtistDetail(n)
	if err != nil {
		return nil, failed(err)
	}
	page := &source.ArtistPage{
		Artist: artist(*ar),
		Tabs: []source.Tab{
			{ID: tabHot, Label: "热门歌曲", Type: source.TypeTrack},
			{ID: tabLatest, Label: "最新歌曲", Type: source.TypeTrack},
			{ID: tabAlbums, Label: "专辑", Type: source.TypeAlbum},
		},
		First: &source.FirstTab{Tab: tabHot, Items: trackItems(ar.Songs)},
	}
	if ar.AccountID != 0 {
		page.Tabs = append(page.Tabs, source.Tab{ID: tabEvents, Label: "动态", Type: source.TypePost})
	}
	return page, nil
}

func (s *Source) User(ctx context.Context, userID string) (*source.UserPage, error) {
	n, err := number(userID)
	if err != nil {
		return nil, err
	}
	u, err := api.UserDetail(n)
	if err != nil {
		return nil, failed(err)
	}
	made, _, err := api.UserPlaylists(n)
	if err != nil {
		return nil, failed(err)
	}
	return &source.UserPage{
		User:  user(*u),
		Tabs:  []source.Tab{{ID: "playlists", Label: "歌单", Type: source.TypePlaylist}},
		First: &source.FirstTab{Tab: "playlists", Items: playlistItems(made)},
	}, nil
}

func (s *Source) LoadTab(ctx context.Context, e source.EntityRef, tab, cursor string) (*source.Page[source.Item], error) {
	n, err := number(e.ID)
	if err != nil {
		return nil, err
	}
	offset := offsetOf(cursor)
	switch {
	case e.Type == source.TypeArtist && tab == tabHot:
		ar, err := api.ArtistDetail(n)
		if err != nil {
			return nil, failed(err)
		}
		return &source.Page[source.Item]{Items: trackItems(ar.Songs)}, nil
	case e.Type == source.TypeArtist && tab == tabLatest:
		songs, err := api.ArtistLatestSongs(n, latestCount)
		if err != nil {
			return nil, failed(err)
		}
		return &source.Page[source.Item]{Items: trackItems(songs)}, nil
	case e.Type == source.TypeArtist && tab == tabAlbums:
		albums, more, err := api.ArtistAlbums(n, albumsPage, offset)
		if err != nil {
			return nil, failed(err)
		}
		return &source.Page[source.Item]{Items: albumItems(albums), Next: next(offset, len(albums), more)}, nil
	case e.Type == source.TypeArtist && tab == tabEvents:
		var x artistExtra
		if len(e.Extra) > 0 {
			if err := json.Unmarshal(e.Extra, &x); err != nil {
				return nil, source.Wrap(source.CodeInvalidArgument, err)
			}
		}
		if x.AccountID == 0 {
			return &source.Page[source.Item]{}, nil
		}
		// The cursor is the time to go on from, in milliseconds; none for
		// the latest.
		before := int64(-1)
		if cursor != "" {
			if before, err = number(cursor); err != nil {
				return nil, err
			}
		}
		events, last, more, err := api.Events(x.AccountID, eventsPage, before)
		if err != nil {
			return nil, failed(err)
		}
		pg := &source.Page[source.Item]{Items: postItems(events)}
		if more && last != 0 {
			pg.Next = id(last)
		}
		return pg, nil
	case e.Type == source.TypeUser && tab == "playlists":
		made, _, err := api.UserPlaylists(n)
		if err != nil {
			return nil, failed(err)
		}
		return &source.Page[source.Item]{Items: playlistItems(made)}, nil
	}
	return nil, source.Fail(source.CodeNotFound, "没有这个页签")
}

func (s *Source) Playlist(ctx context.Context, playlistID, cursor string) (*source.PlaylistPage, error) {
	n, err := number(playlistID)
	if err != nil {
		return nil, err
	}
	pl, err := api.PlaylistDetail(n)
	if err != nil {
		return nil, failed(err)
	}
	return &source.PlaylistPage{Playlist: playlist(*pl), Tracks: tracks(pl.Tracks)}, nil
}

func (s *Source) Lyrics(ctx context.Context, t *source.Track) (*source.Lyrics, error) {
	n, err := number(t.ID)
	if err != nil {
		return nil, err
	}
	lrc, translation, yrc, err := api.Lyrics(n)
	if err != nil {
		return nil, failed(err)
	}
	if lrc == "" && yrc == "" {
		return nil, source.ErrNotFound
	}
	out := &source.Lyrics{LRC: lrc, TranslationLRC: translation}
	if yrc != "" {
		out.Lines = wordTimed(lrc, translation, yrc)
	}
	return out, nil
}

// Account.

func (s *Source) AuthStatus(ctx context.Context) (*source.AuthStatus, error) {
	p, err := api.Account()
	if err != nil {
		return nil, failed(err)
	}
	if p == nil {
		s.forget()
		return &source.AuthStatus{}, nil
	}
	s.mu.Lock()
	s.uid = p.UserID
	s.mu.Unlock()
	return &source.AuthStatus{LoggedIn: true, User: &source.UserRef{ID: id(p.UserID), Name: p.Nickname, Avatar: image(p.AvatarURL)}}, nil
}

func (s *Source) AuthMethods(ctx context.Context) ([]source.LoginMethod, error) {
	return []source.LoginMethod{{ID: "qr", Label: "扫码登录"}}, nil
}

// The QR login is polled every two seconds, as the app always has.
const qrPollMS = 2000

func (s *Source) AuthBegin(ctx context.Context, method string) (*source.Step, error) {
	if method != "qr" {
		return nil, source.Fail(source.CodeInvalidArgument, "不支持这种登录方式")
	}
	key, err := api.QRKey()
	if err != nil {
		return nil, failed(err)
	}
	return &source.Step{
		Type:    source.StepQR,
		Token:   key,
		Content: api.LoginURL(key),
		State:   source.QRWaiting,
		Hint:    "请使用网易云音乐 App 扫码登录",
		PollMS:  qrPollMS,
	}, nil
}

func (s *Source) AuthPoll(ctx context.Context, token string) (*source.Step, error) {
	code, msg, nickname, avatar, err := api.QRCheck(token)
	if err != nil {
		return nil, failed(err)
	}
	qr := func(state string) *source.Step {
		return &source.Step{Type: source.StepQR, Token: token, Content: api.LoginURL(token), State: state, PollMS: qrPollMS}
	}
	switch code {
	case api.QRWaiting:
		return qr(source.QRWaiting), nil
	case api.QRExpired:
		return qr(source.QRExpired), nil
	case api.QRScanned:
		st := qr(source.QRScanned)
		st.Hint = "请在手机上确认登录"
		st.User = &source.UserRef{Name: nickname, Avatar: image(avatar)}
		return st, nil
	case api.QRConfirmed:
		if err := api.SaveSession(); err != nil {
			return nil, source.Wrap(source.CodeInternal, err)
		}
		s.forget()
		status, err := s.AuthStatus(ctx)
		if err != nil {
			return nil, err
		}
		return &source.Step{Type: source.StepDone, User: status.User}, nil
	}
	return nil, source.Fail(source.CodeInternal, msg)
}

func (s *Source) AuthSubmit(ctx context.Context, token, action string, values map[string]string) (*source.Step, error) {
	return nil, source.ErrUnsupported
}

func (s *Source) AuthCancel(ctx context.Context, token string) error { return nil }

func (s *Source) Logout(ctx context.Context) error {
	api.Logout()
	s.forget()
	return nil
}

// Library.

func (s *Source) Playlists(ctx context.Context) (*source.LibraryPlaylists, error) {
	uid, err := s.me()
	if err != nil {
		return nil, err
	}
	made, saved, err := api.UserPlaylists(uid)
	if err != nil {
		return nil, failed(err)
	}
	out := &source.LibraryPlaylists{Owned: playlists(made), Saved: playlists(saved)}
	for i := range out.Owned {
		out.Owned[i].Owned = true
	}
	return out, nil
}

func (s *Source) LikedIDs(ctx context.Context) ([]string, error) {
	uid, err := s.me()
	if err != nil {
		return nil, err
	}
	ids, err := api.LikedIDs(uid)
	if err != nil {
		return nil, failed(err)
	}
	out := make([]string, len(ids))
	for i, n := range ids {
		out[i] = id(n)
	}
	return out, nil
}

// LikedState answers for episodes, which NetEase answers one by one; songs
// are for LikedIDs.
func (s *Source) LikedState(ctx context.Context, ts []source.Track) (map[string]bool, error) {
	out := map[string]bool{}
	var programs []int64
	byProgram := map[int64]string{}
	for _, t := range ts {
		if t.Episode == nil {
			continue
		}
		n, err := number(t.Episode.ID)
		if err != nil {
			return nil, err
		}
		programs = append(programs, n)
		byProgram[n] = t.ID
	}
	if len(programs) > 0 {
		liked, err := api.ProgramsLiked(programs)
		if err != nil {
			return nil, failed(err)
		}
		for n, ok := range liked {
			out[byProgram[n]] = ok
		}
	}
	return out, nil
}

func (s *Source) Like(ctx context.Context, t *source.Track, liked bool) error {
	if t.Episode != nil {
		n, err := number(t.Episode.ID)
		if err != nil {
			return err
		}
		return failedOrNil(api.LikeProgram(n, liked))
	}
	n, err := number(t.ID)
	if err != nil {
		return err
	}
	return failedOrNil(api.Like(n, liked))
}

func failedOrNil(err error) error {
	if err == nil {
		return nil
	}
	return failed(err)
}

func (s *Source) Recent(ctx context.Context, limit int) ([]source.Play, error) {
	plays, err := api.RecentPlays(limit)
	if err != nil {
		return nil, failed(err)
	}
	out := make([]source.Play, len(plays))
	for i, p := range plays {
		out[i] = source.Play{Track: track(p.Track), PlayedMS: p.At.UnixMilli()}
	}
	return out, nil
}

func (s *Source) FollowedArtists(ctx context.Context, cursor string) (*source.Page[source.Artist], error) {
	ars, err := api.FollowedArtists()
	if err != nil {
		return nil, failed(err)
	}
	out := &source.Page[source.Artist]{Items: make([]source.Artist, len(ars))}
	for i, a := range ars {
		out.Items[i] = artist(a)
	}
	return out, nil
}

func (s *Source) Follow(ctx context.Context, artistID string, follow bool) error {
	n, err := number(artistID)
	if err != nil {
		return err
	}
	return failedOrNil(api.FollowArtist(n, follow))
}

func (s *Source) SubscribedPodcasts(ctx context.Context, cursor string) (*source.Page[source.Podcast], error) {
	pods, err := api.SubscribedPodcasts()
	if err != nil {
		return nil, failed(err)
	}
	out := &source.Page[source.Podcast]{Items: make([]source.Podcast, len(pods))}
	for i, p := range pods {
		out.Items[i] = podcast(p)
	}
	return out, nil
}

func (s *Source) SubscribePodcast(ctx context.Context, podcastID string, subscribe bool) error {
	n, err := number(podcastID)
	if err != nil {
		return err
	}
	return failedOrNil(api.SubscribePodcast(n, subscribe))
}

func (s *Source) SavePlaylist(ctx context.Context, playlistID string, saved bool) error {
	n, err := number(playlistID)
	if err != nil {
		return err
	}
	return failedOrNil(api.SubscribePlaylist(n, saved))
}

// Editing the listener's playlists.

func (s *Source) CreatePlaylist(ctx context.Context, title string, private bool) (*source.Playlist, error) {
	pl, err := api.CreatePlaylist(title)
	if err != nil {
		return nil, failed(err)
	}
	out := playlist(*pl)
	out.Owned = true
	return &out, nil
}

func (s *Source) RenamePlaylist(ctx context.Context, playlistID, title string) error {
	n, err := number(playlistID)
	if err != nil {
		return err
	}
	return failedOrNil(api.RenamePlaylist(n, title))
}

func (s *Source) DeletePlaylist(ctx context.Context, playlistID string) error {
	n, err := number(playlistID)
	if err != nil {
		return err
	}
	return failedOrNil(api.DeletePlaylist(n))
}

func (s *Source) AddTracks(ctx context.Context, playlistID string, ts []source.Track) error {
	pid, ids, err := playlistAndTracks(playlistID, ts)
	if err != nil {
		return err
	}
	err = api.AddToPlaylist(pid, ids...)
	if errors.Is(err, api.ErrInPlaylist) {
		return source.Wrap(source.CodeConflict, err)
	}
	return failedOrNil(err)
}

func (s *Source) RemoveTracks(ctx context.Context, playlistID string, ts []source.Track) error {
	pid, ids, err := playlistAndTracks(playlistID, ts)
	if err != nil {
		return err
	}
	return failedOrNil(api.RemoveFromPlaylist(pid, ids...))
}

func playlistAndTracks(playlistID string, ts []source.Track) (int64, []int64, error) {
	pid, err := number(playlistID)
	if err != nil {
		return 0, nil, err
	}
	ids := make([]int64, len(ts))
	for i, t := range ts {
		if ids[i], err = number(t.ID); err != nil {
			return 0, nil, err
		}
	}
	return pid, ids, nil
}

// Feed.

func (s *Source) Home(ctx context.Context) ([]source.Section, error) {
	var (
		wg                           sync.WaitGroup
		featured, charts, rec        []api.Playlist
		daily                        []api.Track
		featuredErr, chartsErr, derr error
	)
	signedIn := false
	if _, err := s.me(); err == nil {
		signedIn = true
	}
	wg.Add(2)
	go func() { defer wg.Done(); featured, featuredErr = api.FeaturedPlaylists(30) }()
	go func() { defer wg.Done(); charts, chartsErr = api.Charts() }()
	if signedIn {
		wg.Add(2)
		go func() { defer wg.Done(); daily, derr = api.DailySongs() }()
		go func() { defer wg.Done(); rec, _ = api.RecommendedPlaylists() }()
	}
	wg.Wait()
	if featuredErr != nil && chartsErr != nil {
		return nil, failed(featuredErr)
	}
	var out []source.Section
	if signedIn && derr == nil && len(daily) > 0 {
		out = append(out, source.Section{ID: "daily", Title: "每日推荐", Style: source.StyleHero, PlayAll: true, Items: trackItems(daily), More: true})
	}
	if len(rec) > 0 {
		out = append(out, source.Section{ID: "recommended", Title: "为你推荐的歌单", Style: source.StyleGrid, Items: playlistItems(rec)})
	}
	if featuredErr == nil {
		out = append(out, source.Section{ID: "featured", Title: "推荐歌单", Style: source.StyleGrid, Items: playlistItems(featured)})
	}
	if chartsErr == nil {
		out = append(out, source.Section{ID: "charts", Title: "排行榜", Style: source.StyleChart, Items: playlistItems(charts), More: true})
	}
	return out, nil
}

func (s *Source) Section(ctx context.Context, sectionID, cursor string) (*source.Section, error) {
	sections, err := s.Home(ctx)
	if err != nil {
		return nil, err
	}
	for i := range sections {
		if sections[i].ID == sectionID {
			return &sections[i], nil
		}
	}
	return nil, source.ErrNotFound
}

// Radio.

func (s *Source) RadioModes(ctx context.Context) ([]source.RadioMode, error) {
	return []source.RadioMode{
		{ID: api.FMDefault, Label: "推荐"},
		{ID: api.FMFamiliar, Label: "熟悉"},
		{ID: api.FMExplore, Label: "探索"},
	}, nil
}

func (s *Source) RadioNext(ctx context.Context, mode string) ([]source.Track, error) {
	ts, err := api.PersonalFM(mode)
	if err != nil {
		return nil, failed(err)
	}
	return tracks(ts), nil
}

func (s *Source) RadioDislike(ctx context.Context, t *source.Track) error {
	n, err := number(t.ID)
	if err != nil {
		return err
	}
	return failedOrNil(api.FMTrash(n))
}

func (s *Source) RadioSimilar(ctx context.Context, q source.SimilarQuery) ([]source.Track, error) {
	seed, err := number(q.Seed.ID)
	if err != nil {
		return nil, err
	}
	start := seed
	if q.Origin != nil {
		if start, err = number(q.Origin.ID); err != nil {
			return nil, err
		}
	}
	pid, err := number(q.PlaylistID)
	if err != nil {
		return nil, err
	}
	ts, err := api.HeartMode(seed, start, pid)
	if err != nil {
		return nil, failed(err)
	}
	return tracks(ts), nil
}

// Comments.

func (s *Source) Comments(ctx context.Context, thread, cur string, limit int) (*source.Comments, error) {
	offset := offsetOf(cur)
	res, err := api.ThreadComments(thread, offset, limit)
	if err != nil {
		return nil, failed(err)
	}
	return &source.Comments{
		Hot:   comments(res.Hot),
		Items: comments(res.Latest),
		Total: res.Total,
		Next:  next(offset, len(res.Latest), res.More),
	}, nil
}

func (s *Source) LikeComment(ctx context.Context, thread, commentID string, liked bool) error {
	n, err := number(commentID)
	if err != nil {
		return err
	}
	return failedOrNil(api.LikeComment(thread, n, liked))
}

// Podcasts.

func (s *Source) PodcastHome(ctx context.Context) ([]source.Section, error) {
	var (
		wg        sync.WaitGroup
		subs, rec []api.Podcast
		hot       []api.Podcast
		hotMore   bool
		recErr    error
		hotErr    error
	)
	signedIn := false
	if _, err := s.me(); err == nil {
		signedIn = true
	}
	wg.Add(2)
	go func() { defer wg.Done(); rec, recErr = api.RecommendedPodcasts() }()
	go func() { defer wg.Done(); hot, hotMore, hotErr = api.HotPodcasts(podcastsPage, 0) }()
	if signedIn {
		wg.Add(1)
		go func() { defer wg.Done(); subs, _ = api.SubscribedPodcasts() }()
	}
	wg.Wait()
	if recErr != nil && hotErr != nil {
		return nil, failed(recErr)
	}
	var out []source.Section
	if len(subs) > 0 {
		out = append(out, source.Section{ID: "subscribed", Title: "我订阅的", Style: source.StyleGrid, Items: podcastItems(subs)})
	}
	if recErr == nil {
		out = append(out, source.Section{ID: "recommended", Title: "推荐播客", Style: source.StyleGrid, Items: podcastItems(rec)})
	}
	if hotErr == nil {
		out = append(out, source.Section{ID: "hot", Title: "热门播客", Style: source.StyleGrid, Items: podcastItems(hot), Next: next(0, len(hot), hotMore), More: hotMore})
	}
	return out, nil
}

func (s *Source) PodcastSection(ctx context.Context, sectionID, cur string) (*source.Section, error) {
	if sectionID != "hot" {
		sections, err := s.PodcastHome(ctx)
		if err != nil {
			return nil, err
		}
		for i := range sections {
			if sections[i].ID == sectionID {
				return &sections[i], nil
			}
		}
		return nil, source.ErrNotFound
	}
	offset := offsetOf(cur)
	hot, more, err := api.HotPodcasts(podcastsPage, offset)
	if err != nil {
		return nil, failed(err)
	}
	return &source.Section{ID: "hot", Title: "热门播客", Style: source.StyleGrid, Items: podcastItems(hot), Next: next(offset, len(hot), more), More: more}, nil
}

func (s *Source) Podcast(ctx context.Context, podcastID, cur string) (*source.PodcastPage, error) {
	n, err := number(podcastID)
	if err != nil {
		return nil, err
	}
	pod, err := api.PodcastDetail(n)
	if err != nil {
		return nil, failed(err)
	}
	offset := offsetOf(cur)
	programs, more, err := api.PodcastPrograms(n, podcastsPage, offset)
	if err != nil {
		return nil, failed(err)
	}
	eps := make([]source.Track, len(programs))
	for i := range programs {
		eps[i] = track(programs[i].Track(pod))
	}
	return &source.PodcastPage{Podcast: podcast(*pod), Episodes: eps, Next: next(offset, len(eps), more)}, nil
}
