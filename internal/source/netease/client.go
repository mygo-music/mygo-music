package netease

import api "github.com/mygo-music/mygo-music/internal/netease"

// What the source asks of the service. Tests of the source replace them, as
// the tests of the app replace theirs, to answer without the network.
var (
	apiRestoreSession       = api.RestoreSession
	apiClearSession         = api.ClearSession
	apiSaveSession          = api.SaveSession
	apiAccount              = api.Account
	apiQRCheck              = api.QRCheck
	apiEvents               = api.Events
	apiHotPodcasts          = api.HotPodcasts
	apiRecommendedPodcasts  = api.RecommendedPodcasts
	apiSubscribedPodcasts   = api.SubscribedPodcasts
	apiProgramsLiked        = api.ProgramsLiked
	apiLikeProgram          = api.LikeProgram
	apiLike                 = api.Like
	apiSearchSuggest        = api.SearchSuggest
	apiFeaturedPlaylists    = api.FeaturedPlaylists
	apiCharts               = api.Charts
	apiDailySongs           = api.DailySongs
	apiRecommendedPlaylists = api.RecommendedPlaylists
)
