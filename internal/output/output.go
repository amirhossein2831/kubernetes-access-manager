package output

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/amirhossein2831/kubernetes-access-manager/internal/domain"
)

type Results struct{ File, Stdout error }
type Writer struct {
	mu           sync.Mutex
	file, stdout io.Writer
	Observe      func(sink string, err error, d time.Duration)
	Emergency    io.Writer
}

func New(file, stdout io.Writer) *Writer { return &Writer{file: file, stdout: stdout} }
func (w *Writer) Write(record any) Results {
	b, err := json.Marshal(record)
	if err != nil {
		return Results{err, err}
	}
	return w.WriteJSON(b)
}
func (w *Writer) WriteJSON(b []byte) Results {
	w.mu.Lock()
	defer w.mu.Unlock()
	line := append(append([]byte{}, b...), '\n')
	result := Results{}
	for _, sink := range []struct {
		name   string
		writer io.Writer
		target *error
	}{{"file", w.file, &result.File}, {"stdout", w.stdout, &result.Stdout}} {
		start := time.Now()
		n, err := sink.writer.Write(line)
		if err == nil && n != len(line) {
			err = io.ErrShortWrite
		}
		*sink.target = err
		if w.Observe != nil {
			w.Observe(sink.name, err, time.Since(start))
		}
		if err != nil && w.Emergency != nil {
			fmt.Fprintf(w.Emergency, "access-monitor: %s output failed\n", sink.name)
		}
	}
	return result
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

type Record struct {
	SchemaVersion string    `json:"schema_version"`
	EventType     string    `json:"event_type"`
	EventID       string    `json:"event_id"`
	SnapshotID    string    `json:"snapshot_id"`
	ObservedAt    time.Time `json:"observed_at"`
	StartedAt     time.Time `json:"started_at"`
	ExpectedUsers int       `json:"expected_users"`
	IdentityBasis string    `json:"identity_basis"`
	domain.UserAccess
}
