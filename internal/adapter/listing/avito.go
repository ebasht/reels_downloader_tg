package listing

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// The item is in the router hydration state: a JSON document inside a JS string.
var avitoStateRe = regexp.MustCompile(`window\.__staticRouterHydrationData\s*=\s*JSON\.parse\(("(?:[^"\\]|\\.)*")\)`)

type avitoState struct {
	LoaderData map[string]json.RawMessage `json:"loaderData"`
}

type avitoBuyerItem struct {
	BuyerItem *struct {
		Item struct {
			Title          string `json:"title"`
			Description    string `json:"description"`
			FormattedPrice struct {
				FormatedString string `json:"formatedString"`
			} `json:"formattedPrice"`
		} `json:"item"`
		GalleryInfo struct {
			Media []struct {
				IsVideo bool              `json:"isVideo"`
				URLs    map[string]string `json:"urls"`
			} `json:"media"`
		} `json:"galleryInfo"`
	} `json:"buyerItem"`
}

func parseAvito(page string) (ad, bool) {
	m := avitoStateRe.FindStringSubmatch(page)
	if m == nil {
		return ad{}, false
	}
	var raw string
	if err := json.Unmarshal([]byte(m[1]), &raw); err != nil {
		return ad{}, false
	}
	var state avitoState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return ad{}, false
	}

	for _, data := range state.LoaderData {
		var v avitoBuyerItem
		if err := json.Unmarshal(data, &v); err != nil || v.BuyerItem == nil {
			continue
		}
		b := v.BuyerItem

		var images []string
		for _, media := range b.GalleryInfo.Media {
			if !media.IsVideo {
				images = appendUnique(images, largestImage(media.URLs))
			}
		}
		return ad{
			title:       htmlToText(b.Item.Title),
			price:       htmlToText(b.Item.FormattedPrice.FormatedString),
			description: htmlToText(b.Item.Description),
			imageURLs:   images,
		}, true
	}
	return ad{}, false
}

// largestImage picks the biggest size from {"1280x960": url, "640x480": url}.
func largestImage(urls map[string]string) string {
	best, bestArea := "", 0
	for size, u := range urls {
		w, h, ok := strings.Cut(size, "x")
		if !ok {
			continue
		}
		wi, _ := strconv.Atoi(w)
		hi, _ := strconv.Atoi(h)
		if area := wi * hi; area > bestArea {
			best, bestArea = u, area
		}
	}
	return best
}
