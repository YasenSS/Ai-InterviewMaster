package aieval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type GoldenCase struct {
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Answer      string         `json:"answer"`
	Question    string         `json:"question"`
	Untrusted   map[string]any `json:"untrusted"`
	ExpectZero  bool           `json:"expect_zero"`
	ExpectBlock bool           `json:"expect_block"`
}

func loadGolden(t *testing.T, name string) GoldenCase {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "golden", "v1", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var item GoldenCase
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	return item
}
