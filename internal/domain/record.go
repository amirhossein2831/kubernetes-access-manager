package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Record struct {
	SchemaVersion string    `json:"schema_version"`
	EventType     string    `json:"event_type"`
	EventID       string    `json:"event_id"`
	SnapshotID    string    `json:"snapshot_id"`
	ObservedAt    time.Time `json:"observed_at"`
	StartedAt     time.Time `json:"started_at"`
	ExpectedUsers int       `json:"expected_users"`
	IdentityBasis string    `json:"identity_basis"`
	UserAccess
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("OS random source unavailable")
	}
	return hex.EncodeToString(b[:])
}

func EventID(snapshot, username string) string {
	b, _ := json.Marshal([]string{snapshot, username})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
