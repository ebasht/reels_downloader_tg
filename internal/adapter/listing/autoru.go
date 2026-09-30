package listing

import "strings"

// parseAutoRu reads an auto.ru listing from its JSON-LD Product. The page
// truncates the seller's text at about 900 characters.
func parseAutoRu(page string) (ad, bool) {
	p := ldJSONOfType(page, "Product", "Car", "Vehicle")
	if p == nil {
		return ad{}, false
	}

	var full, all []string
	images, _ := p["image"].([]any)
	for _, raw := range images {
		img := obj(raw)
		u := str(img["contentUrl"])
		all = appendUnique(all, u)
		if strings.HasSuffix(u, "/1200x900") {
			full = appendUnique(full, u)
		}
	}
	if len(full) == 0 {
		full = all
	}

	// og:description is a ready summary: body, year, mileage, engine, price.
	title := metaProperty(page, "og:description")
	price := ""
	if title == "" {
		title = str(p["name"])
		price = formatRub(number(obj(p["offers"])["price"]))
	}

	return ad{
		title:       title,
		price:       price,
		description: htmlToText(str(p["description"])),
		imageURLs:   full,
	}, true
}
