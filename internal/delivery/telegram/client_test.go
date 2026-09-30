package telegram

import (
	"net/http"
	"strings"
	"testing"
)

func TestRedactingClientHidesToken(t *testing.T) {
	const token = "123456:SECRET-token"
	c := &redactingClient{inner: &http.Client{}, token: token}

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1/bot"+token+"/getMe", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Do(req)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("token leaked: %v", err)
	}
	if !strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("error not redacted: %v", err)
	}
}
