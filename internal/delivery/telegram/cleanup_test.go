package telegram

import (
	"errors"
	"testing"
)

func TestDiffIDs(t *testing.T) {
	got := diffIDs([]int{1, 2, 3, 4}, []int{2, 4})
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("got %v", got)
	}
	if got := diffIDs([]int{1, 2}, nil); len(got) != 2 {
		t.Fatalf("nil keep: %v", got)
	}
}

func TestIsTransientDeleteError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("Bad Request: message to delete not found"), false},
		{errors.New("Forbidden: bot was kicked from the group chat"), false},
		{errors.New("Too Many Requests: retry after 3"), true},
		{errors.New("Post \"https://api.telegram.org\": context deadline exceeded (Client.Timeout)"), true},
		{errors.New("connection reset by peer"), true},
	}
	for _, c := range cases {
		if got := isTransientDeleteError(c.err); got != c.want {
			t.Fatalf("%v: got %v want %v", c.err, got, c.want)
		}
	}
}

func TestMessageIDFromResult(t *testing.T) {
	id, err := messageIDFromResult([]byte(`{"message_id":42,"date":1,"chat":{"id":1,"type":"private"}}`))
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if _, err := messageIDFromResult([]byte(`{}`)); err == nil {
		t.Fatal("expected error for missing id")
	}
}
