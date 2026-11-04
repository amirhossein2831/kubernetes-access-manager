package domain

import (
	"encoding/hex"
	"testing"
)

func TestNewID(t *testing.T) {
	first := NewID()
	if len(first) != 32 {
		t.Fatalf("unexpected ID length: %d", len(first))
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Fatal(err)
	}
	if first == NewID() {
		t.Fatal("generated duplicate IDs")
	}
}

func TestEventIDStableAndDistinct(t *testing.T) {
	first := EventID("snapshot", "alice")
	if first != EventID("snapshot", "alice") || len(first) != 64 {
		t.Fatal("event ID is not a stable SHA-256 identifier")
	}
	if first == EventID("other-snapshot", "alice") || first == EventID("snapshot", "bob") {
		t.Fatal("event ID does not distinguish snapshots and users")
	}
	if EventID("ab", "c") == EventID("a", "bc") {
		t.Fatal("event ID has ambiguous component boundaries")
	}
}
