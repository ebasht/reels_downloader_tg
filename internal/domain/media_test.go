package domain

import "testing"

func TestFindInstagramLink(t *testing.T) {
	tests := []struct {
		name string
		text string
		want InstagramLink
		ok   bool
	}{
		{
			name: "reel with query",
			text: "смотри https://www.instagram.com/reel/Dd4BAPNsnje/?stkn=YjJ5MHc5dnZpaWpn",
			want: InstagramLink{Kind: LinkReel, Shortcode: "Dd4BAPNsnje", URL: "https://www.instagram.com/reel/Dd4BAPNsnje/"},
			ok:   true,
		},
		{
			name: "reels plural",
			text: "https://instagram.com/reels/Abc_-1/",
			want: InstagramLink{Kind: LinkReel, Shortcode: "Abc_-1", URL: "https://www.instagram.com/reel/Abc_-1/"},
			ok:   true,
		},
		{
			name: "post with img_index",
			text: "https://www.instagram.com/p/Ddl-FKUDYTH/?img_index=3&stkn=amRwdGI1M2Nibmxr",
			want: InstagramLink{Kind: LinkPost, Shortcode: "Ddl-FKUDYTH", URL: "https://www.instagram.com/p/Ddl-FKUDYTH/"},
			ok:   true,
		},
		{
			name: "post under username",
			text: "https://www.instagram.com/avtorynok_rostov_/p/Ddl-FKUDYTH/",
			want: InstagramLink{Kind: LinkPost, Shortcode: "Ddl-FKUDYTH", URL: "https://www.instagram.com/p/Ddl-FKUDYTH/"},
			ok:   true,
		},
		{
			name: "profile link",
			text: "https://www.instagram.com/unit_auto/",
			ok:   false,
		},
		{
			name: "no link",
			text: "привет",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FindInstagramLink(tt.text)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
