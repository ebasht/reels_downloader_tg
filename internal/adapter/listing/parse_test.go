package listing

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAutoRu(t *testing.T) {
	page := `<html><head>
<meta property="og:description" content="Седан Audi S4 I (B5) 1999 года, пробег 247 000 км, за 1 050 000 рублей."/>
<script type="application/ld+json">{"@type":"Organization","name":"Авто.ру"}</script>
<script type="application/ld+json">{"@type":"Product","name":"Audi S4 I (B5)","description":"Продаётся Audi S4.\n\nНовая КПП.",
"offers":{"@type":"Offer","price":1050000},
"image":[
 {"contentUrl":"https://avatars.avto.ru/get-autoru-vos/1/a/320x240","width":"320"},
 {"contentUrl":"https://avatars.avto.ru/get-autoru-vos/1/a/1200x900","width":"1200"},
 {"contentUrl":"https://avatars.avto.ru/get-autoru-vos/2/b/320x240","width":"320"},
 {"contentUrl":"https://avatars.avto.ru/get-autoru-vos/2/b/1200x900","width":"1200"}]}</script>
</head></html>`

	a, ok := parseAutoRu(page)
	if !ok {
		t.Fatal("not parsed")
	}
	if a.title != "Седан Audi S4 I (B5) 1999 года, пробег 247 000 км, за 1 050 000 рублей." || a.price != "" {
		t.Errorf("title=%q price=%q", a.title, a.price)
	}
	if a.description != "Продаётся Audi S4.\n\nНовая КПП." {
		t.Errorf("description=%q", a.description)
	}
	want := []string{"https://avatars.avto.ru/get-autoru-vos/1/a/1200x900", "https://avatars.avto.ru/get-autoru-vos/2/b/1200x900"}
	if strings.Join(a.imageURLs, " ") != strings.Join(want, " ") {
		t.Errorf("images=%q", a.imageURLs)
	}
}

func TestParseAvito(t *testing.T) {
	state := map[string]any{"loaderData": map[string]any{
		"0": nil,
		"catalog-or-main-or-item": map[string]any{"buyerItem": map[string]any{
			"item": map[string]any{
				"title":          "Skoda Superb 2.0 AMT, 2017, 177\u00a0000\u00a0км",
				"description":    "<p>Уникальный проект.</p><p>Возможен обмен!<br>Звоните.</p>",
				"formattedPrice": map[string]any{"formatedString": "10 000 000&nbsp;₽"},
			},
			"galleryInfo": map[string]any{"media": []any{
				map[string]any{"isVideo": false, "urls": map[string]any{"640x480": "https://20.img.avito.st/small1", "1280x960": "https://20.img.avito.st/big1"}},
				map[string]any{"isVideo": true, "urls": map[string]any{"1280x960": "https://20.img.avito.st/video"}},
				map[string]any{"isVideo": false, "urls": map[string]any{"1280x960": "https://20.img.avito.st/big2"}},
			}},
		}},
	}}
	inner, _ := json.Marshal(state)
	outer, _ := json.Marshal(string(inner))
	page := `<script>window.__staticRouterHydrationData = JSON.parse(` + string(outer) + `);</script>`

	a, ok := parseAvito(page)
	if !ok {
		t.Fatal("not parsed")
	}
	if a.title != "Skoda Superb 2.0 AMT, 2017, 177 000 км" || a.price != "10 000 000 ₽" {
		t.Errorf("title=%q price=%q", a.title, a.price)
	}
	if a.description != "Уникальный проект.\n\nВозможен обмен!\nЗвоните." {
		t.Errorf("description=%q", a.description)
	}
	if strings.Join(a.imageURLs, " ") != "https://20.img.avito.st/big1 https://20.img.avito.st/big2" {
		t.Errorf("images=%q", a.imageURLs)
	}
}

func TestParseDrom(t *testing.T) {
	page := `<script type="application/ld+json">{"@type":"Car","name":"Mitsubishi Lancer Evolution","vehicleModelDate":"2005",
"offers":{"price":4500000},"mileageFromOdometer":{"value":"77000"},
"image":{"url":"https://s12.auto.drom.ru/photo/v2/AAA/gen600.jpg"}}</script>
<img src="https://s12.auto.drom.ru/photo/v2/AAA/gen1200.jpg"><img src="https://s12.auto.drom.ru/photo/v2/AAA/gen1200.jpg">
<img src="https://s12.auto.drom.ru/photo/v2/BBB/gen1200.jpg">
<div data-ftid="bulletin-description"><div data-ftid="info-full"><span data-ftid="property">Дополнительно: </span>` +
		`<span data-ftid="value">806лс настоящих&#x2026;<br/>АИ100+ метанол!<br/><br/><br/>Поршни – Mahle <br />` + "\n" +
		`Шатуны – Manley<br />` + "\n" + ` Торг.</span></div></div>`

	a, ok := parseDrom(page)
	if !ok {
		t.Fatal("not parsed")
	}
	if a.title != "Mitsubishi Lancer Evolution, 2005, 77 000 км" || a.price != "4 500 000 ₽" {
		t.Errorf("title=%q price=%q", a.title, a.price)
	}
	if a.description != "806лс настоящих…\nАИ100+ метанол!\n\nПоршни – Mahle\nШатуны – Manley\nТорг." {
		t.Errorf("description=%q", a.description)
	}
	if strings.Join(a.imageURLs, " ") != "https://s12.auto.drom.ru/photo/v2/AAA/gen1200.jpg https://s12.auto.drom.ru/photo/v2/BBB/gen1200.jpg" {
		t.Errorf("images=%q", a.imageURLs)
	}
}

func TestFormatRub(t *testing.T) {
	for in, want := range map[int64]string{0: "", 999: "999 ₽", 1000: "1 000 ₽", 4500000: "4 500 000 ₽", 10000000: "10 000 000 ₽"} {
		if got := formatRub(in); got != want {
			t.Errorf("formatRub(%d) = %q, want %q", in, got, want)
		}
	}
}
