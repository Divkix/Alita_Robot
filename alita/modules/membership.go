package modules

import (
	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/stats"
)

// ErrChallengeDisabled signals the challenge adapter is disabled; the caller
// falls through to welcome, matching the historic greetings/captcha behavior.
var ErrChallengeDisabled = errors.New("challenge disabled")

// JoinOutcome is the one verdict membership intake gives a join.
type JoinOutcome int

const (
	JoinIgnore JoinOutcome = iota
	JoinWelcome
	JoinChallenge
	// JoinRemoved: a guard stage removed the member.
	JoinRemoved
	// JoinHeld: a guard stage kept the member from speaking (a mute).
	JoinHeld
)

// StageAnswer is what a guard stage did with a join.
type StageAnswer int

const (
	// StageNotApplicable: the guard has nothing against the member.
	StageNotApplicable StageAnswer = iota
	StageRemoved
	StageHeld
	// StageFailed: the guard tried and could not act; the member goes on to
	// the next stage as if it had not applied.
	StageFailed
)

// JoinStage is one guard of membership intake, run in order before the
// challenge or welcome.
type JoinStage func(user gotgbot.User) StageAnswer

// JoinClaim is who holds a join after the intake tried to claim it.
type JoinClaim int

const (
	// ClaimLost: another update or replica already took this join.
	ClaimLost JoinClaim = iota
	// ClaimShared: this update holds the claim every replica sees.
	ClaimShared
	// ClaimLocal: the shared claim failed, so this replica claimed the join
	// for itself. Guards still run; nothing that could be sent twice is.
	ClaimLocal
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
	Claim      func(chatID, userID int64) JoinClaim
	IsApproved func(chatID, userID int64) bool
	// Stages are the guards, in order. The first that removes or holds the
	// member ends the join.
	Stages    []JoinStage
	Challenge func(user gotgbot.User) error
	Welcome   func(user gotgbot.User) error
	// Probation puts a member who is not approved on probation when the chat
	// has it on. It runs only for the update that won the join claim.
	Probation func(chatID int64, user gotgbot.User)
}

type joinErrorStage uint8

const (
	joinErrorChallenge joinErrorStage = iota
	joinErrorWelcome
)

type joinStageError struct {
	stage joinErrorStage
	cause error
}

func (e joinStageError) Error() string {
	switch e.stage {
	case joinErrorChallenge:
		return "join challenge failed: " + e.cause.Error()
	case joinErrorWelcome:
		return "join welcome failed: " + e.cause.Error()
	default:
		return "join processing failed: " + e.cause.Error()
	}
}

func (e joinStageError) Unwrap() error {
	return e.cause
}

func ProcessSingleJoin(chatID, botID int64, user gotgbot.User, captchaEnabled bool, deps MembershipDeps) (JoinOutcome, error) {
	if user.Id == botID {
		return JoinIgnore, nil
	}
	claim := ClaimShared
	if deps.Claim != nil {
		claim = deps.Claim(chatID, user.Id)
	}
	if claim == ClaimLost {
		return JoinIgnore, nil
	}
	for _, stage := range deps.Stages {
		switch stage(user) {
		case StageRemoved:
			return JoinRemoved, nil
		case StageHeld:
			return JoinHeld, nil
		case StageNotApplicable, StageFailed:
		}
	}
	if claim == ClaimLocal {
		return JoinIgnore, nil
	}
	// Both join updates (the new_chat_members service message and the
	// ChatMemberUpdated) funnel through this claim and only the winner gets
	// past it, so counting here records exactly one join per membership
	// change whichever update arrived first.
	stats.RecordJoin(chatID)
	approved := false
	if deps.IsApproved != nil {
		approved = deps.IsApproved(chatID, user.Id)
	}
	if !approved && deps.Probation != nil {
		deps.Probation(chatID, user)
	}
	switch DecideJoin(JoinInput{IsApproved: approved, CaptchaEnabled: captchaEnabled}) {
	case JoinChallenge:
		if deps.Challenge != nil {
			if err := deps.Challenge(user); err != nil {
				if errors.Is(err, ErrChallengeDisabled) {
					break
				}
				return JoinIgnore, joinStageError{stage: joinErrorChallenge, cause: err}
			}
			return JoinChallenge, nil
		}
		return JoinChallenge, nil
	default:
	}
	if deps.Welcome != nil {
		if err := deps.Welcome(user); err != nil {
			return JoinIgnore, joinStageError{stage: joinErrorWelcome, cause: err}
		}
	}
	return JoinWelcome, nil
}
