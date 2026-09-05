package modules

import (
	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
)

// ErrChallengeDisabled signals the challenge adapter is disabled; the caller
// falls through to welcome, matching the historic greetings/captcha behavior.
var ErrChallengeDisabled = errors.New("challenge disabled")

type JoinOutcome int

const (
	JoinIgnore JoinOutcome = iota
	JoinWelcome
	JoinChallenge
)

type JoinInput struct {
	IsBotSelf      bool
	IsApproved     bool
	CaptchaEnabled bool
}

func DecideJoin(in JoinInput) JoinOutcome {
	if in.IsBotSelf {
		return JoinIgnore
	}
	if in.CaptchaEnabled && !in.IsApproved {
		return JoinChallenge
	}
	return JoinWelcome
}

type MembershipDeps struct {
	Claim      func(chatID, userID int64) bool
	IsApproved func(chatID, userID int64) bool
	Challenge  func(user gotgbot.User) error
	Welcome    func(user gotgbot.User) error
}

func ProcessSingleJoin(chatID, botID int64, user gotgbot.User, captchaEnabled bool, deps MembershipDeps) (JoinOutcome, error) {
	if user.Id == botID {
		return JoinIgnore, nil
	}
	if deps.Claim != nil && !deps.Claim(chatID, user.Id) {
		return JoinIgnore, nil
	}
	approved := false
	if deps.IsApproved != nil {
		approved = deps.IsApproved(chatID, user.Id)
	}
	switch DecideJoin(JoinInput{IsApproved: approved, CaptchaEnabled: captchaEnabled}) {
	case JoinChallenge:
		if deps.Challenge != nil {
			if err := deps.Challenge(user); err != nil {
				if errors.Is(err, ErrChallengeDisabled) {
					break
				}
				return JoinIgnore, err
			}
			return JoinChallenge, nil
		}
		return JoinChallenge, nil
	default:
	}
	if deps.Welcome != nil {
		if err := deps.Welcome(user); err != nil {
			return JoinIgnore, err
		}
	}
	return JoinWelcome, nil
}

func ProcessJoins(chatID, botID int64, members []gotgbot.User, captchaEnabled bool, deps MembershipDeps) []JoinOutcome {
	out := make([]JoinOutcome, 0, len(members))
	for _, m := range members {
		o, err := ProcessSingleJoin(chatID, botID, m, captchaEnabled, deps)
		if err != nil {
			log.Error(err)
		}
		out = append(out, o)
	}
	return out
}
