package telegram

import (
	"strings"
	"testing"
)

func TestSplitText(t *testing.T) {
	text := strings.Repeat("а", 5) + "\n" + strings.Repeat("б", 5)
	got := splitText(text, 8)
	want := []string{strings.Repeat("а", 5) + "\n", strings.Repeat("б", 5)}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %q, want %q", got, want)
	}

	long := strings.Repeat("x", 10)
	if got := splitText(long, 4); len(got) != 3 || strings.Join(got, "") != long {
		t.Fatalf("got %q", got)
	}

	emoji := strings.Repeat("😀", 3) // 2 UTF-16 units each
	if got := splitText(emoji, 4); len(got) != 2 || got[0] != "😀😀" {
		t.Fatalf("got %q", got)
	}
}
