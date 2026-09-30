package domain

import "testing"

func TestFindLink(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Link
		ok   bool
	}{
		{
			name: "reel with query",
			text: "смотри https://www.instagram.com/reel/Dd4BAPNsnje/?stkn=YjJ5MHc5dnZpaWpn",
			want: Link{Kind: LinkReel, Source: SourceInstagram, ID: "Dd4BAPNsnje", URL: "https://www.instagram.com/reel/Dd4BAPNsnje/"},
			ok:   true,
		},
		{
			name: "reels plural",
			text: "https://instagram.com/reels/Abc_-1/",
			want: Link{Kind: LinkReel, Source: SourceInstagram, ID: "Abc_-1", URL: "https://www.instagram.com/reel/Abc_-1/"},
			ok:   true,
		},
		{
			name: "post with img_index",
			text: "https://www.instagram.com/p/Ddl-FKUDYTH/?img_index=3&stkn=amRwdGI1M2Nibmxr",
			want: Link{Kind: LinkPost, Source: SourceInstagram, ID: "Ddl-FKUDYTH", URL: "https://www.instagram.com/p/Ddl-FKUDYTH/"},
			ok:   true,
		},
		{
			name: "post under username",
			text: "https://www.instagram.com/avtorynok_rostov_/p/Ddl-FKUDYTH/",
			want: Link{Kind: LinkPost, Source: SourceInstagram, ID: "Ddl-FKUDYTH", URL: "https://www.instagram.com/p/Ddl-FKUDYTH/"},
			ok:   true,
		},
		{
			name: "auto.ru",
			text: "глянь https://auto.ru/cars/used/sale/audi/s4/1133503750-ef26872f/",
			want: Link{Kind: LinkListing, Source: SourceAutoRu, ID: "1133503750-ef26872f", URL: "https://auto.ru/cars/used/sale/audi/s4/1133503750-ef26872f/"},
			ok:   true,
		},
		{
			name: "auto.ru mobile without trailing slash",
			text: "https://m.auto.ru/cars/used/sale/bmw/5er/1120000000-abc123?from=share",
			want: Link{Kind: LinkListing, Source: SourceAutoRu, ID: "1120000000-abc123", URL: "https://auto.ru/cars/used/sale/bmw/5er/1120000000-abc123/"},
			ok:   true,
		},
		{
			name: "avito with utm",
			text: "https://www.avito.ru/lipetsk/avtomobili/skoda_superb_2.0_amt_2017_177_000_km_8318356809?utm_campaign=native&utm_source=soc_sharing",
			want: Link{Kind: LinkListing, Source: SourceAvito, ID: "8318356809", URL: "https://www.avito.ru/lipetsk/avtomobili/skoda_superb_2.0_amt_2017_177_000_km_8318356809"},
			ok:   true,
		},
		{
			name: "avito mobile at end of text",
			text: "продают https://m.avito.ru/moskva/avtomobili/bmw_5_seriya_2001_1234567890",
			want: Link{Kind: LinkListing, Source: SourceAvito, ID: "1234567890", URL: "https://www.avito.ru/moskva/avtomobili/bmw_5_seriya_2001_1234567890"},
			ok:   true,
		},
		{
			name: "drom",
			text: "https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html",
			want: Link{Kind: LinkListing, Source: SourceDrom, ID: "323106173", URL: "https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html"},
			ok:   true,
		},
		{
			name: "auto.ru without scheme",
			text: "auto.ru/cars/used/sale/renault/clio_rs/1133826284-8036e9d5/",
			want: Link{Kind: LinkListing, Source: SourceAutoRu, ID: "1133826284-8036e9d5", URL: "https://auto.ru/cars/used/sale/renault/clio_rs/1133826284-8036e9d5/"},
			ok:   true,
		},
		{
			name: "instagram without scheme",
			text: "вот www.instagram.com/reel/Dd4BAPNsnje/",
			want: Link{Kind: LinkReel, Source: SourceInstagram, ID: "Dd4BAPNsnje", URL: "https://www.instagram.com/reel/Dd4BAPNsnje/"},
			ok:   true,
		},
		{
			name: "drom without scheme",
			text: "auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html",
			want: Link{Kind: LinkListing, Source: SourceDrom, ID: "323106173", URL: "https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html"},
			ok:   true,
		},
		{
			name: "bare instagram.com post",
			text: "instagram.com/p/Ddl-FKUDYTH/?img_index=2",
			want: Link{Kind: LinkPost, Source: SourceInstagram, ID: "Ddl-FKUDYTH", URL: "https://www.instagram.com/p/Ddl-FKUDYTH/"},
			ok:   true,
		},
		{
			name: "bare avito.ru",
			text: "avito.ru/lipetsk/avtomobili/skoda_superb_2.0_amt_2017_177_000_km_8318356809",
			want: Link{Kind: LinkListing, Source: SourceAvito, ID: "8318356809", URL: "https://www.avito.ru/lipetsk/avtomobili/skoda_superb_2.0_amt_2017_177_000_km_8318356809"},
			ok:   true,
		},
		{
			name: "bare drom.ru",
			text: "смотри drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html",
			want: Link{Kind: LinkListing, Source: SourceDrom, ID: "323106173", URL: "https://drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html"},
			ok:   true,
		},
		{name: "other domain ending in auto.ru", text: "https://myauto.ru/cars/used/sale/audi/s4/1133503750-ef26872f/"},
		{
			name: "first link wins",
			text: "https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html и https://www.instagram.com/reel/Dd4BAPNsnje/",
			want: Link{Kind: LinkListing, Source: SourceDrom, ID: "323106173", URL: "https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html"},
			ok:   true,
		},
		{name: "instagram profile", text: "https://www.instagram.com/unit_auto/"},
		{name: "auto.ru search page", text: "https://auto.ru/cars/audi/s4/all/"},
		{name: "avito category", text: "https://www.avito.ru/lipetsk/avtomobili"},
		{name: "no link", text: "привет"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FindLink(tt.text)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, tt.ok, got)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
