package output

import (
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	derrs "github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/value"
)

var (
	encTestUUID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	encTestTime = time.Date(2026, 4, 25, 12, 30, 0, 0, time.UTC)
)

func encTestObject() *value.Object {
	o := value.NewObject()
	o.Set("a", value.Int(1))
	o.Set("b", value.Str("x"))
	return o
}

// TestEncodeValueJSONAllKinds exercises every value.Kind through encodeValueJSON
// and asserts the JSON-marshaled result so unexported wrapper types
// (jsonNumber, orderedJSONObject) are covered without reaching into them.
func TestEncodeValueJSONAllKinds(t *testing.T) {
	cases := []struct {
		name string
		in   value.Value
		want string
	}{
		{"null", value.Null(), `null`},
		{"bool", value.Bool(true), `true`},
		{"int", value.Int(-7), `-7`},
		{"float", value.Float(1.5), `1.5`},
		{"string", value.Str("hi \"q\""), `"hi \"q\""`},
		{"uuid", value.UUID(encTestUUID), `"11111111-1111-4111-8111-111111111111"`},
		{"time", value.Time(encTestTime), `"2026-04-25T12:30:00Z"`},
		{"decimal", value.Dec(decimal.RequireFromString("12.30")), `12.3`},
		{"empty list", value.List(nil), `[]`},
		{"list", value.List([]value.Value{value.Int(1), value.Str("a")}), `[1,"a"]`},
		{"empty object", value.Obj(value.NewObject()), `{}`},
		{"object", value.Obj(encTestObject()), `{"a":1,"b":"x"}`},
		{"nested", value.List([]value.Value{value.Obj(encTestObject())}), `[{"a":1,"b":"x"}]`},
		{"nil object", value.Value{Kind: value.KindObject, O: nil}, `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := encodeValueJSON(tc.in)
			if err != nil {
				t.Fatalf("encodeValueJSON(%s): %v", tc.name, err)
			}
			b, err := json.Marshal(enc)
			if err != nil {
				t.Fatalf("marshal(%s): %v", tc.name, err)
			}
			if string(b) != tc.want {
				t.Fatalf("%s = %s, want %s", tc.name, b, tc.want)
			}
		})
	}
}

func TestEncodeValueJSONUnknownKind(t *testing.T) {
	_, err := encodeValueJSON(value.Value{Kind: value.Kind(99)})
	if err == nil {
		t.Fatal("encodeValueJSON unknown kind: want error, got nil")
	}
	if !stderrors.Is(err, derrs.ErrGeneration) {
		t.Fatalf("error = %v, want generation kind", err)
	}
}

// TestRenderValueScalarAllKinds exercises every value.Kind through
// renderValueScalar.
func TestRenderValueScalarAllKinds(t *testing.T) {
	cases := []struct {
		name string
		in   value.Value
		want string
	}{
		{"null", value.Null(), ``},
		{"bool true", value.Bool(true), `true`},
		{"bool false", value.Bool(false), `false`},
		{"int", value.Int(-7), `-7`},
		{"float", value.Float(1.5), `1.5`},
		{"string", value.Str("hi"), `hi`},
		{"uuid", value.UUID(encTestUUID), `11111111-1111-4111-8111-111111111111`},
		{"time", value.Time(encTestTime), `2026-04-25T12:30:00Z`},
		{"decimal", value.Dec(decimal.RequireFromString("12.30")), `12.3`},
		{"list", value.List([]value.Value{value.Int(1), value.Str("a")}), `[1,"a"]`},
		{"object", value.Obj(encTestObject()), `{"a":1,"b":"x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderValueScalar(tc.in)
			if err != nil {
				t.Fatalf("renderValueScalar(%s): %v", tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestRenderValueScalarUnknownKind(t *testing.T) {
	_, err := renderValueScalar(value.Value{Kind: value.Kind(99)})
	if err == nil {
		t.Fatal("renderValueScalar unknown kind: want error, got nil")
	}
	if !stderrors.Is(err, derrs.ErrGeneration) {
		t.Fatalf("error = %v, want generation kind", err)
	}
}
