//go:build integration

package listing

import (
	"context"
	"testing"
	"time"

	"video_download_bot/internal/domain"
)

// Run with: go test -tags integration -v ./internal/adapter/listing/

func TestFetchListings(t *testing.T) {
	links := []string{
		"https://auto.ru/cars/used/sale/audi/s4/1133503750-ef26872f/",
		"https://www.avito.ru/lipetsk/avtomobili/skoda_superb_2.0_amt_2017_177_000_km_8318356809",
		"https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html",
	}
	f := NewFetcher(30 * time.Second)

	for _, raw := range links {
		link, ok := domain.FindLink(raw)
		if !ok || link.Kind != domain.LinkListing {
			t.Fatalf("%s: not a listing link", raw)
		}
		t.Run(string(link.Source), func(t *testing.T) {
			l, err := f.FetchListing(context.Background(), link)
			if err != nil {
				t.Skipf("listing unavailable (sold or site blocked the request?): %v", err)
			}
			if l.Title == "" || l.Description == "" || len(l.Images) == 0 {
				t.Fatalf("incomplete listing: title=%q desc=%d photos=%d", l.Title, len(l.Description), len(l.Images))
			}
			t.Logf("%d photos, caption %d chars:\n%.300s", len(l.Images), len([]rune(l.Caption())), l.Caption())
		})
	}
}
