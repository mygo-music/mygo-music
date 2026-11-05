// Package netease is NetEase Cloud Music as a source: it answers the calls
// of the source package with what the client in internal/netease gets from
// the service. It is the one source built into the app, and the code that
// becomes the official plugin.
package netease

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"strconv"
	"sync"
	"time"

	api "github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// PluginID is the source's ID, which the tracks it gives carry.
const PluginID = "netease"

// Source is NetEase Cloud Music.
type Source struct {
	mu   sync.Mutex
	once sync.Once
	uid  int64 // who is signed in, once known; 0 for nobody
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

// New returns the source. It reads the session the last run kept when it is
// first asked who is signed in, not here: that reads the keychain.
func New() *Source { return &Source{} }

// restore reads the kept session, once.
func (s *Source) restore() { s.once.Do(func() { apiRestoreSession() }) }

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
		source.TypeTrack, source.TypeArtist, source.TypeAlbum, source.TypePlaylist, source.TypeUser, source.TypePodcast,
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
	searchPage   = 30
)

// me returns who is signed in, asking the service the first time. Nobody
// is ErrUnauthenticated.
func (s *Source) me() (int64, error) {
	s.restore()
	// Held while the service is asked: a Logout waits for the answer, and so
	// can't be undone by it.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.uid != 0 {
		return s.uid, nil
	}
	p, err := apiAccount()
	if err != nil {
		return 0, failed(err)
	}
	if p == nil {
		return 0, source.ErrUnauthenticated
	}
	s.uid = p.UserID
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

// failed makes what the client failed with a source error. The service's own
// codes tell a listener who isn't signed in, a thing that isn't there and
// being asked too often from a failure to reach it.
func failed(err error) error {
	var e *api.APIError
	if !errors.As(err, &e) {
		return source.Wrap(source.CodeNetwork, err)
	}
	switch e.Code {
	case api.CodeNotSignedIn:
		return source.Wrap(source.CodeUnauthenticated, err)
	case api.CodeNotFound:
		return source.Wrap(source.CodeNotFound, err)
	case api.CodeTooFrequent:
		return source.Wrap(source.CodeRateLimited, err)
	}
	return source.Wrap(source.CodeInternal, err)
}

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
	limit := q.Limit
	if limit <= 0 {
		limit = searchPage
	}
	res, err := api.SearchPage(q.Query, kind, limit, offset)
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
	res, err := apiSearchSuggest(query)
	if err != nil {
		return nil, failed(err)
	}
	ars := make([]api.ArtistProfile, len(res.Artists))
	for i, a := range res.Artists {
		ars[i] = api.ArtistProfile{Artist: a}
	}
	groups := map[string]source.SuggestGroup{
		"songs":     {Type: source.TypeTrack, Items: trackItems(res.Songs)},
		"artists":   {Type: source.TypeArtist, Items: artistItems(ars)},
		"albums":    {Type: source.TypeAlbum, Items: albumItems(res.Albums)},
		"playlists": {Type: source.TypePlaylist, Items: playlistItems(res.Playlists)},
	}
	var out []source.SuggestGroup
	// The kinds in the order the service ranks them, then any it left out.
	for _, kind := range append(slices.Clone(res.Order), "songs", "artists", "albums", "playlists") {
		if g, ok := groups[kind]; ok && len(g.Items) > 0 {
			out = append(out, g)
		}
		delete(groups, kind) // listed once, if also in the order
	}
	return out, nil
}

func (s *Source) Resolve(ctx context.Context, t *source.Track, quality string) (*source.Stream, error) {
	n, err := number(t.ID)
	if err != nil {
		return nil, err
	}
	st, err := api.SongURL(n, quality)
	switch {
	case errors.Is(err, api.ErrUnavailable), errors.Is(err, api.ErrUnsupportedFormat):
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

	// The tab of a user's page.
	tabPlaylists = "playlists"
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
			{ID: tabHot, Label: "热门", Type: source.TabTracks},
			{ID: tabLatest, Label: "最新", Type: source.TabTracks},
			{ID: tabAlbums, Label: "专辑", Type: source.TabAlbums},
		},
		First: &source.FirstTab{Tab: tabHot, Items: trackItems(ar.Songs)},
	}
	if ar.AccountID != 0 {
		page.Tabs = append(page.Tabs, source.Tab{ID: tabEvents, Label: "动态", Type: source.TabPosts})
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
		Tabs:  []source.Tab{{ID: tabPlaylists, Label: "歌单", Type: source.TabPlaylists}},
		First: &source.FirstTab{Tab: tabPlaylists, Items: playlistItems(publicMine(made, n))},
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
		events, last, more, err := apiEvents(x.AccountID, eventsPage, before)
		if err != nil {
			return nil, failed(err)
		}
		pg := &source.Page[source.Item]{Items: postItems(events)}
		// A page that starts no earlier than the last would load forever.
		if more && len(events) > 0 && last != 0 && (before == -1 || last < before) {
			pg.Next = id(last)
		}
		return pg, nil
	case e.Type == source.TypeUser && tab == tabPlaylists:
		made, _, err := api.UserPlaylists(n)
		if err != nil {
			return nil, failed(err)
		}
		return &source.Page[source.Item]{Items: playlistItems(publicMine(made, n))}, nil
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
	s.restore()
	p, err := apiAccount()
	if err != nil {
		return nil, failed(err)
	}
	if p == nil {
		// The session expired, or there was none: forget it, in the keychain too.
		apiClearSession()
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
const (
	qrPollMS = 2000
	qrHint   = "打开网易云音乐 App，扫一扫登录"
)

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
		Hint:    qrHint,
		PollMS:  qrPollMS,
	}, nil
}

func (s *Source) AuthPoll(ctx context.Context, token string) (*source.Step, error) {
	code, msg, nickname, avatar, err := apiQRCheck(token)
	if err != nil {
		return nil, failed(err)
	}
	qr := func(state string) *source.Step {
		return &source.Step{Type: source.StepQR, Token: token, Content: api.LoginURL(token), State: state, PollMS: qrPollMS}
	}
	switch code {
	case api.QRWaiting:
		st := qr(source.QRWaiting)
		st.Hint = qrHint
		return st, nil
	case api.QRExpired:
		st := qr(source.QRExpired)
		st.Hint = "二维码已失效"
		return st, nil
	case api.QRScanned:
		st := qr(source.QRScanned)
		st.Hint = "请在手机上确认登录"
		st.User = &source.UserRef{Name: nickname, Avatar: image(avatar)}
		return st, nil
	case api.QRConfirmed:
		// Signed in for this run even if the session can't be kept for the next.
		if err := apiSaveSession(); err != nil {
			log.Println("netease: keeping the session:", err)
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
		liked, err := apiProgramsLiked(programs)
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
		return failedOrNil(apiLikeProgram(n, liked))
	}
	n, err := number(t.ID)
	if err != nil {
		return err
	}
	return failedOrNil(apiLike(n, liked))
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
		out[i] = source.Play{Track: track(p.Track), PlayedMS: p.At.UnixMilli(), Device: device(p.Device)}
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
	pods, err := apiSubscribedPodcasts()
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

// Home loads what the home page shows. The daily songs are asked for signed
// out too, when they are what many listen to; the playlists picked for the
// listener, only signed in.
func (s *Source) Home(ctx context.Context) ([]source.Section, error) {
	var (
		wg                     sync.WaitGroup
		featured, charts, rec  []api.Playlist
		daily                  []api.Track
		featuredErr, chartsErr error
	)
	_, err := s.me()
	signedIn := err == nil
	wg.Add(3)
	go func() { defer wg.Done(); featured, featuredErr = apiFeaturedPlaylists(30) }()
	go func() { defer wg.Done(); charts, chartsErr = apiCharts() }()
	go func() { defer wg.Done(); daily, _ = apiDailySongs() }()
	if signedIn {
		wg.Add(1)
		go func() { defer wg.Done(); rec, _ = apiRecommendedPlaylists() }()
	}
	wg.Wait()
	if featuredErr != nil && chartsErr != nil {
		return nil, failed(featuredErr)
	}
	if featuredErr != nil {
		featured = nil
	}
	if chartsErr != nil {
		charts = nil
	}
	return homeSections(daily, rec, featured, charts, time.Now()), nil
}

// officialCharts is how many charts the home page shows; the rest are on the
// page of the charts.
const officialCharts = 4

// homeSections puts what the home page loads in its sections, in the order
// the page shows them.
func homeSections(daily []api.Track, rec, featured, charts []api.Playlist, now time.Time) []source.Section {
	var out []source.Section
	if len(daily) > 0 {
		out = append(out, source.Section{
			ID: sectionDaily, Title: "每日推荐", Style: source.StyleHero, PlayAll: true,
			Items: trackItems(daily), More: true, ExpiresMS: nextDailyAt(now).UnixMilli(),
		})
	}
	if len(rec) > 0 {
		out = append(out, source.Section{ID: sectionRecommended, Title: "今日为你推荐", Style: source.StyleGrid, Items: playlistItems(rec)})
	}
	if len(charts) > 0 {
		out = append(out, source.Section{
			ID: sectionCharts, Title: "官方榜单", Style: source.StyleChart,
			Items: playlistItems(charts[:min(officialCharts, len(charts))]), More: len(charts) > officialCharts,
		})
	}
	if len(featured) > 0 {
		out = append(out, source.Section{ID: sectionFeatured, Title: "推荐歌单", Style: source.StyleGrid, Items: playlistItems(featured)})
	}
	return out
}

// The sections of the home page.
const (
	sectionDaily       = "daily"
	sectionRecommended = "recommended"
	sectionCharts      = "charts"
	sectionFeatured    = "featured"
)

// beijing is NetEase's time, without daylight saving.
var beijing = time.FixedZone("CST", 8*60*60)

// nextDailyAt is when NetEase next picks the day's songs anew: at 6:00,
// Beijing time.
func nextDailyAt(now time.Time) time.Time {
	b := now.In(beijing)
	at := time.Date(b.Year(), b.Month(), b.Day(), 6, 0, 0, 0, beijing)
	if !at.After(b) {
		at = at.AddDate(0, 0, 1)
	}
	return at
}

// Section loads one section whole: the daily songs, or all the charts, which
// the home page shows four of.
func (s *Source) Section(ctx context.Context, sectionID, cursor string) (*source.Section, error) {
	switch sectionID {
	case sectionDaily:
		daily, err := apiDailySongs()
		if err != nil {
			return nil, failed(err)
		}
		return &source.Section{ID: sectionDaily, Title: "每日推荐", Style: source.StyleHero, PlayAll: true, Items: trackItems(daily), ExpiresMS: nextDailyAt(time.Now()).UnixMilli()}, nil
	case sectionCharts:
		charts, err := apiCharts()
		if err != nil {
			return nil, failed(err)
		}
		return &source.Section{ID: sectionCharts, Title: "排行榜", Style: source.StyleChart, Items: playlistItems(charts)}, nil
	}
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

// publicMine keeps the playlists uid made and made public, which is what a
// user's page lists.
func publicMine(lists []api.Playlist, uid int64) []api.Playlist {
	var out []api.Playlist
	for _, pl := range lists {
		if pl.Creator.UserID == uid && pl.Privacy != 10 {
			out = append(out, pl)
		}
	}
	return out
}

// device names the kind of device NetEase calls an os.
func device(os string) string {
	switch os {
	case "iphone", "android", "ipad", "harmony":
		return source.DevicePhone
	case "osx", "pc", "mac", "windows", "linux":
		return source.DeviceDesktop
	case "web":
		return source.DeviceWeb
	}
	return ""
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
	go func() { defer wg.Done(); rec, recErr = apiRecommendedPodcasts() }()
	go func() { defer wg.Done(); hot, hotMore, hotErr = apiHotPodcasts(podcastsPage, 0) }()
	if signedIn {
		wg.Add(1)
		go func() { defer wg.Done(); subs, _ = apiSubscribedPodcasts() }()
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
	hot, more, err := apiHotPodcasts(podcastsPage, offset)
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
		eps[i].Episode.PublishedMS = programs[i].CreateTime
	}
	return &source.PodcastPage{Podcast: podcast(*pod), Episodes: eps, Next: next(offset, len(eps), more)}, nil
}
