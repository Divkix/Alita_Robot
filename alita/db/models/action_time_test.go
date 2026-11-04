package models

import "testing"

func TestValidActionTime(t *testing.T) {
	for _, seconds := range []int{60, 600, 366 * 24 * 60 * 60} {
		if !ValidActionTime(seconds) {
			t.Errorf("ValidActionTime(%d) = false, want true", seconds)
		}
	}
	for _, seconds := range []int{-1, 0, 59, 366*24*60*60 + 1} {
		if ValidActionTime(seconds) {
			t.Errorf("ValidActionTime(%d) = true, want false", seconds)
		}
	}
}
