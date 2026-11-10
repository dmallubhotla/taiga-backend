package main

import (
	"os"
	"testing"
)

func TestReadPasswordFromPipe(t *testing.T) {
	for input, want := range map[string]string{
		"hunter2hunter2\n":   "hunter2hunter2",
		"hunter2hunter2\r\n": "hunter2hunter2",
		"no-newline-at-eof":  "no-newline-at-eof",
		"first\nsecond\n":    "first",
	} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.WriteString(input)
		_ = w.Close()

		got, err := readPassword(r)
		_ = r.Close()
		if err != nil || got != want {
			t.Errorf("readPassword(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}
