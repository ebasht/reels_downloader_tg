package web

import (
	"net/url"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"
)

func TestAllowed(t *testing.T) {
	c := NewClient(time.Second, UserAgentTelegram, "en", "instagram.com", "cdninstagram.com")
	tests := map[string]bool{
		"https://www.instagram.com/p/x/":                   true,
		"https://instagram.com/p/x/":                       true,
		"https://scontent-ams2-1.cdninstagram.com/v/a.jpg": true,
		"http://www.instagram.com/p/x/":                    false,
		"https://evil.com/a.jpg":                           false,
		"https://instagram.com.evil.com/a.jpg":             false,
		"https://evilcdninstagram.com/a.jpg":               false,
		"https://169.254.169.254/latest/meta-data":         false,
	}
	for raw, want := range tests {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Allowed(u); got != want {
			t.Errorf("Allowed(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestResponseTextDecodesWindows1251(t *testing.T) {
	encoded, err := charmap.Windows1251.NewEncoder().String("Продажа авто")
	if err != nil {
		t.Fatal(err)
	}
	r := Response{Body: []byte(encoded), ContentType: "text/html; charset=windows-1251"}
	got, err := r.Text()
	if err != nil || got != "Продажа авто" {
		t.Fatalf("got %q, err %v", got, err)
	}

	utf := Response{Body: []byte("Продажа"), ContentType: "text/html; charset=utf-8"}
	if got, _ := utf.Text(); got != "Продажа" {
		t.Fatalf("utf-8: got %q", got)
	}
}
