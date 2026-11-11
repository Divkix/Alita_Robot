package modules

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
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
// challenge or welcome. ctx is the join's update; its effective sender added
// user.
type JoinStage func(bot *gotgbot.Bot, ctx *ext.Context, user gotgbot.User) StageAnswer

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
	// Count sees every join the intake claimed, before the stages and
	// whatever they do with it: a join wave is a raid whoever is in it.
	Count func(user gotgbot.User)
	// Stages are the guards, in order, run on Bot and Ctx. The first that
	// removes or holds the member ends the join.
	Bot       *gotgbot.Bot
	Ctx       *ext.Context
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
	if deps.Count != nil {
		deps.Count(user)
	}
	for _, stage := range deps.Stages {
		switch stage(deps.Bot, deps.Ctx, user) {
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

// The join claim: one per chat and member, so one update on one replica
// processes a join. Only membership intake takes, keeps or releases it — its
// handlers in greetings.go and ProcessSingleJoin through deps.Claim. A guard
// never touches it; the federation watcher reports a ban through
// markJoinHandled.
var (
	recentJoinProcessTTL = 5 * time.Second
	recentJoinProcessing sync.Map
)

type joinClaim struct{ exp time.Time }

func joinClaimKey(chatID, userID int64) string {
	return fmt.Sprintf("alita:recentJoinProcessing:%d:%d", chatID, userID)
}

// claimJoin is membership intake's join claim. A Redis error falls back to a
// claim local to this replica, which lets the guard stages run but not the
// challenge or welcome, so an outage neither lets a banned user in nor greets
// anyone twice.
func claimJoin(chatID, userID int64) JoinClaim {
	claimed, err := tryClaimJoin(chatID, userID)
	if err != nil {
		log.Debugf("[Membership] Redis SETNX failed for join dedupe key %s, using the local claim: %v", joinClaimKey(chatID, userID), err)
		if claimJoinLocally(joinClaimKey(chatID, userID)) {
			return ClaimLocal
		}
		return ClaimLost
	}
	if claimed {
		return ClaimShared
	}
	return ClaimLost
}

// markJoinHandled claims userID's join for a guard that removed them on
// another update, so a later update of that join does nothing.
func markJoinHandled(chatID, userID int64) {
	claimJoin(chatID, userID)
}

// tryClaimJoin takes the join claim in Redis, or in this
// replica without Redis. It returns the Redis error instead of a decision.
func tryClaimJoin(chatID, userID int64) (bool, error) {
	key := joinClaimKey(chatID, userID)

	if rdb := cache.GetRedisClient(); rdb != nil {
		_, err := rdb.SetArgs(cache.Context, key, true, redis.SetArgs{
			Mode: "NX",
			TTL:  recentJoinProcessTTL,
		}).Result()
		if err == nil {
			return true, nil
		}
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, err
	}
	return claimJoinLocally(key), nil
}

func claimJoinLocally(key string) bool {
	now := time.Now()
	for {
		newClaim := joinClaim{exp: now.Add(recentJoinProcessTTL)}
		actual, loaded := recentJoinProcessing.LoadOrStore(key, newClaim)
		if !loaded {
			expireRecentJoinClaim(key, newClaim)
			return true
		}
		if c, ok := actual.(joinClaim); ok && now.Before(c.exp) {
			return false
		}
		newClaim = joinClaim{exp: time.Now().Add(recentJoinProcessTTL)}
		if recentJoinProcessing.CompareAndSwap(key, actual, newClaim) {
			expireRecentJoinClaim(key, newClaim)
			return true
		}
		now = time.Now()
	}
}

func expireRecentJoinClaim(key string, claim joinClaim) {
	time.AfterFunc(recentJoinProcessTTL, func() {
		recentJoinProcessing.CompareAndDelete(key, claim)
	})
}

func clearJoinClaim(chatID, userID int64) {
	key := joinClaimKey(chatID, userID)

	if rdb := cache.GetRedisClient(); rdb != nil {
		if err := rdb.Del(cache.Context, key).Err(); err != nil {
			log.Debugf("[Membership] Failed to clear shared join dedupe key %s: %v", key, err)
		}
	}

	recentJoinProcessing.Delete(key)
}
