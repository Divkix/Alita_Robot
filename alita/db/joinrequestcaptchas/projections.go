package joinrequestcaptchas

import "github.com/divkix/Alita_Robot/alita/db/cache"

// The cache projections of join-request captchas: one attempt per
// (chat, user) and one per attempt id.
var (
	AttemptProjection   = cache.Projection{Name: "join_request_captcha"}
	AttemptIDProjection = cache.Projection{Name: "join_request_captcha_id"}
)
