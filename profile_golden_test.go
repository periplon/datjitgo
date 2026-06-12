package datjit_test

import (
	"bytes"
	"os"
	"testing"

	datjit "github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/datjittest"
)

// profileSchema returns the shared profiles fixture schema. The same schema
// is covered under the default (realistic) profile by TestFixtures
// (testdata/golden/profiles.json); the tests here pin the edge and hostile
// outputs for it.
func profileSchema(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/fixtures/profiles.yaml")
	if err != nil {
		t.Fatalf("read profiles fixture: %v", err)
	}
	return string(b)
}

// TestProfileGoldens pins the edge and hostile profile outputs for the
// profiles fixture. Run `go test -run TestProfileGoldens -update .` to
// regenerate after an intentional boundary-table change.
func TestProfileGoldens(t *testing.T) {
	schema := profileSchema(t)
	for _, tc := range []struct {
		profile string
		golden  string
	}{
		{profile: "edge", golden: "testdata/golden/profile_edge.json"},
		{profile: "hostile", golden: "testdata/golden/profile_hostile.json"},
	} {
		t.Run(tc.profile, func(t *testing.T) {
			opts := []datjit.Option{datjit.WithSeed(42), datjit.WithProfile(tc.profile)}
			if *update {
				datjittest.UpdateGoldenJSON(t, tc.golden, schema, opts...)
				return
			}
			datjittest.AssertGoldenJSON(t, tc.golden, schema, opts...)
		})
	}
}

// TestProfileDeterminism asserts that the same schema + seed + profile
// produces byte-identical output across runs, for every profile.
func TestProfileDeterminism(t *testing.T) {
	schema := profileSchema(t)
	for _, profile := range []string{"realistic", "edge", "hostile"} {
		t.Run(profile, func(t *testing.T) {
			opts := []datjit.Option{datjit.WithSeed(7), datjit.WithProfile(profile)}
			a, err := datjit.GenerateJSONString(schema, opts...)
			if err != nil {
				t.Fatalf("first generate: %v", err)
			}
			b, err := datjit.GenerateJSONString(schema, opts...)
			if err != nil {
				t.Fatalf("second generate: %v", err)
			}
			if !bytes.Equal(a, b) {
				t.Fatal("same seed + profile produced different bytes")
			}
		})
	}
}

// TestRealisticProfileMatchesDefault asserts the cardinal profile rule: an
// explicit realistic profile (or the empty string) is byte-identical to not
// configuring a profile at all — zero extra RNG draws.
func TestRealisticProfileMatchesDefault(t *testing.T) {
	schema := profileSchema(t)
	base, err := datjit.GenerateJSONString(schema, datjit.WithSeed(42))
	if err != nil {
		t.Fatalf("baseline generate: %v", err)
	}
	for _, profile := range []string{"", "realistic"} {
		got, err := datjit.GenerateJSONString(schema, datjit.WithSeed(42), datjit.WithProfile(profile))
		if err != nil {
			t.Fatalf("profile %q generate: %v", profile, err)
		}
		if !bytes.Equal(base, got) {
			t.Fatalf("profile %q output differs from default output", profile)
		}
	}
}

// TestWithProfileValidation covers the option's accept/reject surface.
func TestWithProfileValidation(t *testing.T) {
	for _, ok := range []string{"", "realistic", "edge", "hostile"} {
		if _, err := datjit.New(datjit.WithProfile(ok)); err != nil {
			t.Fatalf("WithProfile(%q) unexpectedly failed: %v", ok, err)
		}
	}
	for _, bad := range []string{"EDGE", "fuzz", "realistic "} {
		if _, err := datjit.New(datjit.WithProfile(bad)); err == nil {
			t.Fatalf("WithProfile(%q) unexpectedly succeeded", bad)
		}
	}
}
