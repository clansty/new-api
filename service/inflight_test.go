package service

import "testing"

func TestInflightCounter_whenRequestBeginsAndEnds(t *testing.T) {
	userID := 101
	channelID := 202

	finishUser := BeginUserInflight(userID)
	finishChannel := BeginChannelInflight(channelID)

	if got := GetUserInflightCount(userID); got != 1 {
		t.Fatalf("GetUserInflightCount() = %d, want 1", got)
	}
	if got := GetChannelInflightCount(channelID); got != 1 {
		t.Fatalf("GetChannelInflightCount() = %d, want 1", got)
	}

	finishUser()
	finishChannel()

	if got := GetUserInflightCount(userID); got != 0 {
		t.Fatalf("GetUserInflightCount() after finish = %d, want 0", got)
	}
	if got := GetChannelInflightCount(channelID); got != 0 {
		t.Fatalf("GetChannelInflightCount() after finish = %d, want 0", got)
	}
}

func TestInflightCounter_whenSameIDHasMultipleRequests(t *testing.T) {
	userID := 303

	finishFirst := BeginUserInflight(userID)
	finishSecond := BeginUserInflight(userID)

	if got := GetUserInflightCount(userID); got != 2 {
		t.Fatalf("GetUserInflightCount() = %d, want 2", got)
	}

	finishFirst()
	if got := GetUserInflightCount(userID); got != 1 {
		t.Fatalf("GetUserInflightCount() after first finish = %d, want 1", got)
	}

	finishSecond()
	if got := GetUserInflightCount(userID); got != 0 {
		t.Fatalf("GetUserInflightCount() after second finish = %d, want 0", got)
	}
}

func TestInflightCounter_whenIDIsInvalid(t *testing.T) {
	finishUser := BeginUserInflight(0)
	finishChannel := BeginChannelInflight(-1)
	finishUser()
	finishChannel()

	if got := GetUserInflightCount(0); got != 0 {
		t.Fatalf("GetUserInflightCount() = %d, want 0", got)
	}
	if got := GetChannelInflightCount(-1); got != 0 {
		t.Fatalf("GetChannelInflightCount() = %d, want 0", got)
	}
}

func TestInflightCounter_whenFinishIsCalledTwice(t *testing.T) {
	userID := 404

	finish := BeginUserInflight(userID)
	finish()
	finishSecond := BeginUserInflight(userID)
	finish()

	if got := GetUserInflightCount(userID); got != 1 {
		t.Fatalf("GetUserInflightCount() after duplicate finish = %d, want 1", got)
	}

	finishSecond()
	if got := GetUserInflightCount(userID); got != 0 {
		t.Fatalf("GetUserInflightCount() after cleanup = %d, want 0", got)
	}
}
