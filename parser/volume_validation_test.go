package parser

import (
	"strings"
	"testing"
)

func TestParseRejectsNegativeVolume(t *testing.T) {
	src := `
domain: bad_volume
volume:
  User: -1
entities:
  User:
    id: uuid
`
	_, err := New().Parse(strings.NewReader(src), "bad.yaml")
	if err == nil {
		t.Fatal("Parse succeeded; expected negative volume error")
	}
}

func TestParseRejectsDescendingVolumeRange(t *testing.T) {
	src := `
domain: bad_volume
volume:
  User: 10..2
entities:
  User:
    id: uuid
`
	_, err := New().Parse(strings.NewReader(src), "bad.yaml")
	if err == nil {
		t.Fatal("Parse succeeded; expected descending volume range error")
	}
}
