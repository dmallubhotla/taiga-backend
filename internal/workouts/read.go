package workouts

import (
	"bytes"
	"io"
)

// ReadActivityFile reads a FIT file, an Endomondo export's JSON or a TCX file, told apart by content rather than name.
func ReadActivityFile(r io.Reader) (*Run, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// Every FIT header carries ".FIT" at bytes 8–11.
	if len(content) >= 12 && string(content[8:12]) == ".FIT" {
		return ReadFitFile(bytes.NewReader(content))
	}
	if trimmed := bytes.TrimSpace(content); len(trimmed) > 0 {
		switch trimmed[0] {
		case '[':
			return ReadEndomondoFile(bytes.NewReader(content))
		case '<':
			return ReadTCXFile(bytes.NewReader(content))
		}
	}
	return ReadFitFile(bytes.NewReader(content))
}
