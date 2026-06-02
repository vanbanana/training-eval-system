package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// AssertStatus checks that the response has the expected status code.
func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}

// AssertJSON checks that the response has Content-Type application/json.
func AssertJSON(t *testing.T, resp *http.Response) {
	t.Helper()
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" && ct != "application/json; charset=utf-8" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
}

// DecodeJSON reads and decodes the response body into v.
func DecodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
}

// AuthHeader returns an Authorization header value for the given token.
func AuthHeader(token string) string {
	return "Bearer " + token
}
