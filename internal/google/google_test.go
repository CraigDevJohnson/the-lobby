package google

import (
	"bytes"
	"errors"
	"testing"
)

func TestSealOpensOnlyForItsMember(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	sealed, err := Seal(key, "a@example.com", "refresh-token")
	if err != nil || bytes.Contains([]byte(sealed), []byte("refresh-token")) {
		t.Fatalf("sealed %q, %v", sealed, err)
	}
	if got, err := Open(key, "a@example.com", sealed); err != nil || got != "refresh-token" {
		t.Fatalf("open = %q, %v", got, err)
	}
	// Copied to another Member's item, or opened with another key, it
	// doesn't open.
	if _, err := Open(key, "b@example.com", sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("another Member: %v", err)
	}
	if _, err := Open(bytes.Repeat([]byte{2}, 32), "a@example.com", sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("another key: %v", err)
	}
	if _, err := Open(key, "a@example.com", "not sealed"); !errors.Is(err, ErrSealed) {
		t.Fatalf("garbage: %v", err)
	}
	if _, err := Seal(key[:16], "a@example.com", "x"); err == nil {
		t.Fatal("a short key sealed")
	}
}
