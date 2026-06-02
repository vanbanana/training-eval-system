package service

import (
	"testing"

	"pgregory.net/rapid"
)

// Property 14: Path traversal rejection
func TestProperty_PathTraversalRejection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate malicious filenames
		malicious := rapid.SampledFrom([]string{
			"../etc/passwd",
			"..\\windows\\system32",
			"../../secret.txt",
			"/etc/shadow",
			"C:\\Windows\\System32\\config",
			"file\x00.txt",
			"..%2f..%2fetc%2fpasswd",
			"foo/../../../bar",
			"test\\..\\..\\secret",
		}).Draw(t, "malicious")

		err := validateFilename(malicious)
		if err == nil {
			t.Fatalf("path traversal should be rejected: %q", malicious)
		}
	})
}

// Property 14 supplement: Valid filenames should pass
func TestProperty_ValidFilenamesAccepted(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate valid filenames
		name := rapid.StringMatching(`[a-zA-Z0-9_-]{1,20}\.(pdf|docx|png|jpg)`).Draw(t, "name")

		err := validateFilename(name)
		if err != nil {
			t.Fatalf("valid filename should be accepted: %q, got error: %v", name, err)
		}
	})
}
