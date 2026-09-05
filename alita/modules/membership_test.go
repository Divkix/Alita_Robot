package modules

import (
	"errors"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func testMembershipDeps(claim func(int64, int64) bool, approved map[int64]bool, challengeErr error) (MembershipDeps, *[]int64, *[]int64) {
	var challenged, welcomed []int64
	return MembershipDeps{
		Claim: claim,
		IsApproved: func(_, uid int64) bool {
			return approved[uid]
		},
		Challenge: func(u gotgbot.User) error {
			if challengeErr != nil {
				return challengeErr
			}
			challenged = append(challenged, u.Id)
			return nil
		},
		Welcome: func(u gotgbot.User) error {
			welcomed = append(welcomed, u.Id)
			return nil
		},
	}, &challenged, &welcomed
}

func TestDecideJoin(t *testing.T) {
	cases := []struct {
		name string
		in   JoinInput
		want JoinOutcome
	}{
		{"bot self ignored", JoinInput{IsBotSelf: true, CaptchaEnabled: true}, JoinIgnore},
		{"challenge when captcha on unapproved", JoinInput{CaptchaEnabled: true}, JoinChallenge},
		{"welcome approved bypass", JoinInput{CaptchaEnabled: true, IsApproved: true}, JoinWelcome},
		{"welcome captcha off", JoinInput{}, JoinWelcome},
	}
	for _, c := range cases {
		if got := DecideJoin(c.in); got != c.want {
			t.Fatalf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestProcessSingleJoinDuplicate(t *testing.T) {
	claimed := map[int64]bool{}
	deps, challenged, welcomed := testMembershipDeps(
		func(_, uid int64) bool {
			if claimed[uid] {
				return false
			}
			claimed[uid] = true
			return true
		},
		nil, nil,
	)
	u := gotgbot.User{Id: 11, FirstName: "N"}
	if o, _ := ProcessSingleJoin(1, 999, u, false, deps); o != JoinWelcome {
		t.Fatalf("first join = %d, want welcome", o)
	}
	if o, _ := ProcessSingleJoin(1, 999, u, false, deps); o != JoinIgnore {
		t.Fatalf("duplicate join = %d, want ignore", o)
	}
	if len(*challenged) != 0 || len(*welcomed) != 1 {
		t.Fatalf("challenged=%v welcomed=%v, want 0/1", *challenged, *welcomed)
	}
}

func TestProcessSingleJoinApprovedBypass(t *testing.T) {
	deps, challenged, welcomed := testMembershipDeps(nil, map[int64]bool{7: true}, nil)
	o, err := ProcessSingleJoin(1, 999, gotgbot.User{Id: 7}, true, deps)
	if err != nil || o != JoinWelcome {
		t.Fatalf("approved join = %d,%v want welcome,nil", o, err)
	}
	if len(*challenged) != 0 || len(*welcomed) != 1 {
		t.Fatalf("challenged=%v welcomed=%v", *challenged, *welcomed)
	}
}

func TestProcessSingleJoinChallengeRouting(t *testing.T) {
	deps, challenged, _ := testMembershipDeps(nil, nil, nil)
	o, _ := ProcessSingleJoin(1, 999, gotgbot.User{Id: 5}, true, deps)
	if o != JoinChallenge || len(*challenged) != 1 {
		t.Fatalf("challenge routing = %d challenged=%v", o, *challenged)
	}
	if o, _ := ProcessSingleJoin(1, 999, gotgbot.User{Id: 999}, true, deps); o != JoinIgnore {
		t.Fatalf("bot self = %d, want ignore", o)
	}
}

func TestProcessSingleJoinChallengeDisabledFallsThrough(t *testing.T) {
	deps, challenged, welcomed := testMembershipDeps(nil, nil, ErrChallengeDisabled)
	o, err := ProcessSingleJoin(1, 999, gotgbot.User{Id: 5}, true, deps)
	if err != nil || o != JoinWelcome {
		t.Fatalf("disabled challenge = %d,%v want welcome,nil", o, err)
	}
	if len(*challenged) != 0 || len(*welcomed) != 1 {
		t.Fatalf("challenged=%v welcomed=%v", *challenged, *welcomed)
	}
}

func TestProcessSingleJoinChallengeError(t *testing.T) {
	boom := errors.New("boom")
	deps, _, _ := testMembershipDeps(nil, nil, boom)
	if _, err := ProcessSingleJoin(1, 999, gotgbot.User{Id: 5}, true, deps); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestProcessJoinsMultiMember(t *testing.T) {
	claimed := map[int64]bool{2: true}
	deps, challenged, welcomed := testMembershipDeps(
		func(_, uid int64) bool {
			if claimed[uid] {
				return false
			}
			claimed[uid] = true
			return true
		},
		map[int64]bool{3: true}, nil,
	)
	members := []gotgbot.User{{Id: 2}, {Id: 3}, {Id: 4}}
	out := ProcessJoins(1, 999, members, true, deps)
	if len(out) != 3 || out[0] != JoinIgnore || out[1] != JoinWelcome || out[2] != JoinChallenge {
		t.Fatalf("outcomes = %v", out)
	}
	if len(*challenged) != 1 || len(*welcomed) != 1 {
		t.Fatalf("challenged=%v welcomed=%v", *challenged, *welcomed)
	}
}
