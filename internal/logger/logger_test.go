package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

type broken struct{}

func (broken) Write([]byte) (int, error) {
	return 0, errors.New("disk full")
}

func TestIndependentSinks(t *testing.T) {
	for _, badFile := range []bool{true, false} {
		var good bytes.Buffer
		var w *Writer
		if badFile {
			w = NewWriter(broken{}, &good)
		} else {
			w = NewWriter(&good, broken{})
		}
		r := w.Write(map[string]string{"event_type": "access_snapshot"})
		if (r.File == nil) == badFile || (r.Stdout == nil) != badFile {
			t.Fatalf("wrong sink status %+v", r)
		}
		if !strings.Contains(good.String(), "access_snapshot") {
			t.Fatal("other sink suppressed")
		}
	}
}

func TestConcurrentJSONLines(t *testing.T) {
	var file, stdout bytes.Buffer
	w := NewWriter(&file, &stdout)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w.Write(map[string]string{"line": "one"}) }()
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSpace(file.String()), "\n")
	if len(lines) != 50 || file.String() != stdout.String() {
		t.Fatal("lost or different output")
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatal("interleaved JSON")
		}
	}
}
