package listing

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	ldJSONRe = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
	// A literal newline right after <br> is source formatting, not a second break.
	lineBreakRe   = regexp.MustCompile(`(?i)(?:<br\s*/?>|</div>|</li>)\r?\n?`)
	paragraphRe   = regexp.MustCompile(`(?i)</p>`)
	tagRe         = regexp.MustCompile(`<[^>]+>`)
	blankLinesRe  = regexp.MustCompile(`\n{3,}`)
	lineEdgeWSRe  = regexp.MustCompile(`[ \t]*\n[ \t]*`)
	metaContentRe = regexp.MustCompile(`(?s)<meta\b[^>]*>`)
	propertyRe    = regexp.MustCompile(`\bproperty="([^"]+)"`)
	contentRe     = regexp.MustCompile(`\bcontent="([^"]*)"`)
)

// htmlToText turns an HTML fragment into plain text, keeping line breaks.
func htmlToText(s string) string {
	s = paragraphRe.ReplaceAllString(s, "\n\n")
	s = lineBreakRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	// Some sites double-escape entities (&amp;nbsp;).
	s = html.UnescapeString(html.UnescapeString(s))
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = lineEdgeWSRe.ReplaceAllString(s, "\n")
	s = blankLinesRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// formatRub formats 4500000 as "4 500 000 ₽".
func formatRub(price int64) string {
	if price <= 0 {
		return ""
	}
	digits := strconv.FormatInt(price, 10)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	b.WriteString(" ₽")
	return b.String()
}

// ldJSONObjects returns all JSON-LD objects on the page, flattening arrays.
func ldJSONObjects(page string) []map[string]any {
	var out []map[string]any
	for _, m := range ldJSONRe.FindAllStringSubmatch(page, -1) {
		var v any
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &v); err != nil {
			continue
		}
		switch t := v.(type) {
		case map[string]any:
			out = append(out, t)
		case []any:
			for _, item := range t {
				if obj, ok := item.(map[string]any); ok {
					out = append(out, obj)
				}
			}
		}
	}
	return out
}

// ldJSONOfType returns the first JSON-LD object whose @type is one of types.
func ldJSONOfType(page string, types ...string) map[string]any {
	for _, obj := range ldJSONObjects(page) {
		t, _ := obj["@type"].(string)
		for _, want := range types {
			if t == want {
				return obj
			}
		}
	}
	return nil
}

// metaProperty returns the content of <meta property="name">, in any attribute order.
func metaProperty(page, name string) string {
	for _, tag := range metaContentRe.FindAllString(page, -1) {
		p := propertyRe.FindStringSubmatch(tag)
		if p == nil || p[1] != name {
			continue
		}
		if c := contentRe.FindStringSubmatch(tag); c != nil {
			return htmlToText(c[1])
		}
	}
	return ""
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// number reads a JSON-LD number that may be encoded as a number or a string.
func number(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		return 0
	}
}

func appendUnique(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}
