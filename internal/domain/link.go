package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type LinkKind int

const (
	LinkReel LinkKind = iota + 1
	LinkPost
	// LinkListing is a car sale listing: photos plus the ad text.
	LinkListing
)

type Source string

const (
	SourceInstagram Source = "instagram"
	SourceAutoRu    Source = "autoru"
	SourceAvito     Source = "avito"
	SourceDrom      Source = "drom"
)

type Link struct {
	Kind   LinkKind
	Source Source
	// ID is the Instagram shortcode or the listing ID on its site.
	ID  string
	URL string
}

type linkPattern struct {
	re    *regexp.Regexp
	parse func(m []string) Link
}

var linkPatterns = []linkPattern{
	{
		re: regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.|m\.)?instagram\.com/(?:[A-Za-z0-9_.]+/)?(p|reels?|tv)/([A-Za-z0-9_-]+)`),
		parse: func(m []string) Link {
			kind, path := LinkReel, "reel"
			if strings.EqualFold(m[1], "p") {
				kind, path = LinkPost, "p"
			}
			return Link{
				Kind:   kind,
				Source: SourceInstagram,
				ID:     m[2],
				URL:    fmt.Sprintf("https://www.instagram.com/%s/%s/", path, m[2]),
			}
		},
	},
	{
		// https://auto.ru/cars/used/sale/audi/s4/1133503750-ef26872f/
		re: regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.|m\.)?auto\.ru/((?:[a-z0-9_-]+/)+?sale/(?:[a-z0-9_-]+/)*?(\d+-[0-9a-f]+))/?`),
		parse: func(m []string) Link {
			return Link{
				Kind:   LinkListing,
				Source: SourceAutoRu,
				ID:     m[2],
				URL:    "https://auto.ru/" + strings.ToLower(m[1]) + "/",
			}
		},
	},
	{
		// https://www.avito.ru/lipetsk/avtomobili/skoda_superb_2.0_amt_2017_177_000_km_8318356809
		re: regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.|m\.)?avito\.ru/((?:[a-z0-9_-]+/){2}[a-z0-9_.%-]*?_(\d{6,}))(?:[?#/\s]|$)`),
		parse: func(m []string) Link {
			return Link{
				Kind:   LinkListing,
				Source: SourceAvito,
				ID:     m[2],
				URL:    "https://www.avito.ru/" + m[1],
			}
		},
	},
	{
		// https://auto.drom.ru/moscow/mitsubishi/lancer_evolution/323106173.html
		re: regexp.MustCompile(`(?i)\b(?:https?://)?((?:[a-z0-9-]+\.)?drom\.ru)/((?:[a-z0-9_-]+/)*(\d{6,})\.html)`),
		parse: func(m []string) Link {
			return Link{
				Kind:   LinkListing,
				Source: SourceDrom,
				ID:     m[3],
				URL:    "https://" + strings.ToLower(m[1]) + "/" + m[2],
			}
		},
	},
}

// FindLink returns the first supported link in text.
func FindLink(text string) (Link, bool) {
	link, _, _, ok := findLink(text)
	return link, ok
}

// SplitLink returns the first supported link in text and the rest of the
// text with the whole URL token removed.
func SplitLink(text string) (Link, string, bool) {
	link, start, end, ok := findLink(text)
	if !ok {
		return Link{}, "", false
	}
	// The pattern may stop before a query string; drop the token up to whitespace.
	if i := strings.IndexFunc(text[end:], unicode.IsSpace); i >= 0 {
		end += i
	} else {
		end = len(text)
	}
	comment := strings.Join(strings.Fields(text[:start]+" "+text[end:]), " ")
	return link, comment, true
}

func findLink(text string) (Link, int, int, bool) {
	best, bestPos, bestEnd := Link{}, -1, -1
	for _, p := range linkPatterns {
		loc := p.re.FindStringSubmatchIndex(text)
		if loc == nil || (bestPos >= 0 && loc[0] >= bestPos) {
			continue
		}
		m := make([]string, len(loc)/2)
		for i := range m {
			if loc[2*i] >= 0 {
				m[i] = text[loc[2*i]:loc[2*i+1]]
			}
		}
		best, bestPos, bestEnd = p.parse(m), loc[0], loc[1]
	}
	return best, bestPos, bestEnd, bestPos >= 0
}

// Name is the site name shown to users.
func (s Source) Name() string {
	switch s {
	case SourceInstagram:
		return "Instagram"
	case SourceAutoRu:
		return "auto.ru"
	case SourceAvito:
		return "Авито"
	case SourceDrom:
		return "Дром"
	default:
		return string(s)
	}
}
