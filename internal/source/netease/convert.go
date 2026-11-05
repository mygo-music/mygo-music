package netease

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	api "github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// What the conversions give the app of NetEase: its numbers as the strings
// of IDs, its picture addresses as the templates the app sizes.

func id(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

// number reads an ID back.
func number(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, source.Wrap(source.CodeInvalidArgument, err)
	}
	return n, nil
}

// image makes a picture address a template the app gives a size to: the
// CDN resizes a picture by its param.
func image(pic string) string {
	if pic == "" {
		return ""
	}
	return strings.Replace(pic, "http://", "https://", 1) + "?param={size}y{size}"
}

func artistRefs(as []api.Artist) []source.ArtistRef {
	out := make([]source.ArtistRef, len(as))
	for i, a := range as {
		out[i] = source.ArtistRef{ID: id(a.ID), Name: a.Name}
	}
	return out
}

func extra(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func track(t api.Track) source.Track {
	out := source.Track{
		Plugin:        PluginID,
		ID:            id(t.ID),
		Title:         t.Name,
		Artists:       artistRefs(t.Artists),
		Album:         source.AlbumRef{ID: id(t.Album.ID), Name: t.Album.Name, Cover: image(t.Album.PicURL)},
		DurationMS:    t.Duration,
		Aliases:       t.Alias,
		Reason:        t.Reason,
		CommentThread: t.Thread(),
		URL:           t.URL(),
	}
	switch {
	case t.ID == 0:
		// An episode with no audio of its own to stream.
		out.SetUnplayable("这期节目没有音频")
	case !t.Playable() && t.Privilege != nil && t.Privilege.St < 0:
		out.SetUnplayable("已下架")
	case !t.Playable():
		out.SetUnplayable("需要会员或单独购买，或已下架")
	}
	if t.ProgramID > 0 {
		out.Kind = source.KindEpisode
		out.Episode = &source.Episode{ID: id(t.ProgramID), Description: t.Description}
		out.Episode.Podcast.ID = id(t.PodcastID)
		out.Episode.Podcast.Title = t.Album.Name
	}
	return out
}

func tracks(ts []api.Track) []source.Track {
	out := make([]source.Track, len(ts))
	for i, t := range ts {
		out[i] = track(t)
	}
	return out
}

func album(a api.AlbumProfile) source.Album {
	return source.Album{
		ID:          id(a.ID),
		Title:       a.Name,
		Artists:     artistRefs(a.Artists),
		Cover:       image(a.PicURL),
		ReleasedMS:  a.PublishTime,
		TrackCount:  a.Size,
		Description: a.Description,
	}
}

// artistExtra is what the source keeps of an artist for the tabs of their
// page: the account their events are posted by.
type artistExtra struct {
	AccountID int64 `json:"account_id,omitempty"`
}

// distinct returns ss without empty strings, nor any after its first: an
// artist's translated name is often among their aliases too.
func distinct(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func artist(a api.ArtistProfile) source.Artist {
	// The square picture, else the wide one.
	avatar := a.Avatar
	if avatar == "" {
		avatar = a.PicURL
	}
	out := source.Artist{
		ID:     id(a.ID),
		Name:   a.Name,
		Avatar: image(avatar),
		// The translated name first, as the page reads them.
		Aliases:     distinct(append([]string{a.Trans}, a.Alias...)),
		Description: a.BriefDesc,
		Followed:    a.Followed,
		TrackCount:  a.MusicSize,
		AlbumCount:  a.AlbumSize,
	}
	if a.AccountID != 0 {
		out.Extra = extra(artistExtra{AccountID: a.AccountID})
	}
	return out
}

func playlist(p api.Playlist) source.Playlist {
	out := source.Playlist{
		ID:          id(p.ID),
		Title:       p.Name,
		Cover:       image(p.Cover()),
		Description: p.Description,
		TrackCount:  p.TrackCount,
		PlayCount:   int64(p.PlayCount),
		Creator:     source.UserRef{ID: id(p.Creator.UserID), Name: p.Creator.Nickname},
		UpdateText:  p.UpdateFreq,
		Private:     p.Privacy == 10,
		Subscribed:  p.Subscribed,
	}
	if p.SpecialType == 5 {
		out.Role = source.RoleLiked
	}
	for _, l := range p.Preview {
		out.Preview = append(out.Preview, source.PreviewLine{Title: l.First, Subtitle: l.Second})
	}
	return out
}

func playlists(ps []api.Playlist) []source.Playlist {
	out := make([]source.Playlist, len(ps))
	for i, p := range ps {
		out[i] = playlist(p)
	}
	return out
}

func user(u api.User) source.User {
	return source.User{
		ID: id(u.UserID), Name: u.Nickname, Avatar: image(u.AvatarURL), Description: u.Signature,
		FollowerCount: int64(u.Followeds), FollowingCount: int64(u.Follows),
	}
}

func userRef(u api.User) source.UserRef {
	return source.UserRef{ID: id(u.UserID), Name: u.Nickname, Avatar: image(u.AvatarURL)}
}

func podcast(p api.Podcast) source.Podcast {
	return source.Podcast{
		ID:              id(p.ID),
		Title:           p.Name,
		Cover:           image(p.PicURL),
		Description:     p.Description,
		Category:        p.Category,
		Host:            userRef(p.DJ),
		EpisodeCount:    p.ProgramCount,
		SubscriberCount: p.SubCount,
		PlayCount:       int64(p.PlayCount),
		Subscribed:      p.Subed,
		Tagline:         p.RcmdText,
		LatestEpisode:   p.LastProgram,
	}
}

func comment(c api.Comment) source.Comment {
	out := source.Comment{
		ID:        id(c.ID),
		User:      source.UserRef{ID: id(c.User.ID), Name: c.User.Nickname, Avatar: image(c.User.AvatarURL)},
		Text:      c.Content,
		TimeMS:    c.Time,
		LikeCount: c.LikedCount,
		Liked:     c.Liked,
		Location:  c.IPLocation.Location,
	}
	if len(c.BeReplied) > 0 {
		out.ReplyTo = &source.Reply{UserName: c.BeReplied[0].User.Nickname, Text: c.BeReplied[0].Content}
	}
	return out
}

func comments(cs []api.Comment) []source.Comment {
	out := make([]source.Comment, len(cs))
	for i, c := range cs {
		out[i] = comment(c)
	}
	return out
}

// post makes an event of an account's feed a post.
func post(e api.Event) source.Post {
	p := source.Post{
		ID:                  id(e.ID),
		Author:              userRef(e.User),
		TimeMS:              e.Time,
		Text:                e.Content.Msg,
		Forwarded:           e.Content.Forwarded,
		LikeCount:           e.Info.LikedCount,
		CommentCount:        e.Info.CommentCount,
		ShareCount:          e.Info.ShareCount,
		HasUnsupportedMedia: e.Content.Video,
	}
	for _, pic := range e.Pictures {
		p.Images = append(p.Images, image(pic.SquareURL))
	}
	switch c := e.Content; {
	case c.Song != nil:
		t := track(*c.Song)
		p.Attachment = &source.Item{Type: source.TypeTrack, Track: &t}
	case c.Album != nil:
		a := album(*c.Album)
		p.Attachment = &source.Item{Type: source.TypeAlbum, Album: &a}
	case c.Artist != nil:
		a := artist(*c.Artist)
		p.Attachment = &source.Item{Type: source.TypeArtist, Artist: &a}
	case c.Playlist != nil:
		pl := playlist(*c.Playlist)
		p.Attachment = &source.Item{Type: source.TypePlaylist, Playlist: &pl}
	}
	return p
}

// Items of each kind.

func trackItems(ts []api.Track) []source.Item {
	out := make([]source.Item, len(ts))
	for i, t := range ts {
		v := track(t)
		out[i] = source.Item{Type: source.TypeTrack, Track: &v}
	}
	return out
}

func albumItems(as []api.AlbumProfile) []source.Item {
	out := make([]source.Item, len(as))
	for i, a := range as {
		v := album(a)
		out[i] = source.Item{Type: source.TypeAlbum, Album: &v}
	}
	return out
}

func artistItems(as []api.ArtistProfile) []source.Item {
	out := make([]source.Item, len(as))
	for i, a := range as {
		v := artist(a)
		out[i] = source.Item{Type: source.TypeArtist, Artist: &v}
	}
	return out
}

func playlistItems(ps []api.Playlist) []source.Item {
	out := make([]source.Item, len(ps))
	for i, p := range ps {
		v := playlist(p)
		out[i] = source.Item{Type: source.TypePlaylist, Playlist: &v}
	}
	return out
}

func userItems(us []api.User) []source.Item {
	out := make([]source.Item, len(us))
	for i, u := range us {
		v := user(u)
		out[i] = source.Item{Type: source.TypeUser, User: &v}
	}
	return out
}

func podcastItems(ps []api.Podcast) []source.Item {
	out := make([]source.Item, len(ps))
	for i, p := range ps {
		v := podcast(p)
		out[i] = source.Item{Type: source.TypePodcast, Podcast: &v}
	}
	return out
}

func postItems(es []api.Event) []source.Item {
	out := make([]source.Item, len(es))
	for i, e := range es {
		v := post(e)
		out[i] = source.Item{Type: source.TypePost, Post: &v}
	}
	return out
}
