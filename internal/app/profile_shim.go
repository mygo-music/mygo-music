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
	return &netease.Profile{UserID: id, Nickname: u.Name, AvatarURL: u.Avatar}
}

// sizedPic sizes a picture whichever form it comes in: a source's template
// or a plain address that NetEase's client gives. The pages that read the
// client's types still give the second.
func sizedPic(url string, px int) string {
	if strings.Contains(url, "{size}") {
		return source.Sized(url, px)
	}
	return netease.Sized(url, px)
}
