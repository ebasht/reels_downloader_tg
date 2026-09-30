package domain

import "strings"

// Listing is a car sale ad from a classifieds site.
type Listing struct {
	Title       string
	Price       string
	Description string
	Images      [][]byte
}

// Caption is the message text sent with the listing photos.
func (l Listing) Caption() string {
	var parts []string
	head := strings.TrimSpace(strings.Join(nonEmpty(l.Title, l.Price), "\n"))
	if head != "" {
		parts = append(parts, head)
	}
	if d := strings.TrimSpace(l.Description); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, "\n\n")
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
