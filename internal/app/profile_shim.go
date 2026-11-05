package app

import (
	"strconv"
	"strings"

	"github.com/mygo-music/mygo-music/internal/netease"
	"github.com/mygo-music/mygo-music/internal/source"
)

// profileOf is who a source says is signed in, in the form the pages read
// until they read a source's own: a temporary shim, deleted when they do.
func profileOf(u *source.UserRef) *netease.Profile {
	if u == nil {
		return nil
	}
	id, _ := strconv.ParseInt(u.ID, 10, 64)
	return &netease.Profile{UserID: id, Nickname: u.Name, AvatarURL: rawPic(u.Avatar)}
}

// rawPic is a picture's address without the template that sizes it, which
// the pages that size pictures themselves still add.
func rawPic(url string) string {
	before, _, _ := strings.Cut(url, "?param={size}")
	return before
}
