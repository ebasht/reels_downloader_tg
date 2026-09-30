package listing

import (
	"regexp"
	"strings"
)

var (
	dromFullTextRe  = regexp.MustCompile(`(?s)data-ftid="info-full".*?data-ftid="value"[^>]*>(.*?)</span>`)
	dromDescBlockRe = regexp.MustCompile(`(?s)data-ftid="bulletin-description".*?data-ftid="value"[^>]*>(.*?)</span>`)
	dromPhotoRe     = regexp.MustCompile(`https://[a-z0-9.-]*drom\.ru/photo/v2/[A-Za-z0-9_-]+/gen1200\.jpg`)
)

func parseDrom(page string) (ad, bool) {
	car := ldJSONOfType(page, "Car", "Vehicle", "Product")
	if car == nil {
		return ad{}, false
	}

	title := str(car["name"])
	if year := str(car["vehicleModelDate"]); year != "" {
		title += ", " + year
	}
	if km := number(obj(car["mileageFromOdometer"])["value"]); km > 0 {
		title += ", " + strings.TrimSuffix(formatRub(km), " ₽") + " км"
	}

	var description string
	if m := dromFullTextRe.FindStringSubmatch(page); m != nil {
		description = htmlToText(m[1])
	} else if m := dromDescBlockRe.FindStringSubmatch(page); m != nil {
		description = htmlToText(m[1])
	}

	var images []string
	for _, u := range dromPhotoRe.FindAllString(page, -1) {
		images = appendUnique(images, u)
	}
	if len(images) == 0 {
		if u := str(obj(car["image"])["url"]); u != "" {
			images = []string{u}
		}
	}

	return ad{
		title:       strings.TrimSpace(title),
		price:       formatRub(number(obj(car["offers"])["price"])),
		description: description,
		imageURLs:   images,
	}, true
}
