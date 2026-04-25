package generator

import (
	"strings"
	"testing"

	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/value"
)

func TestGenerateSemanticKnownTags(t *testing.T) {
	eng := newEngine()
	rng := NewRand(123)
	tags := []model.Semantic{
		{Namespace: "person", Tag: "full"},
		{Namespace: "person", Tag: "first"},
		{Namespace: "person", Tag: "last"},
		{Namespace: "person", Tag: "username"},
		{Namespace: "person", Tag: "prefix"},
		{Namespace: "person", Tag: "suffix"},
		{Namespace: "person", Tag: "bio"},
		{Namespace: "person", Tag: "avatar"},
		{Namespace: "person", Tag: "gender"},
		{Namespace: "person", Tag: "dob"},
		{Namespace: "person", Tag: "age"},
		{Namespace: "email"},
		{Namespace: "phone"},
		{Namespace: "url"},
		{Namespace: "url", Tag: "image"},
		{Namespace: "ipv4"},
		{Namespace: "ipv6"},
		{Namespace: "mac"},
		{Namespace: "address", Tag: "full"},
		{Namespace: "address", Tag: "street"},
		{Namespace: "address", Tag: "city"},
		{Namespace: "address", Tag: "state"},
		{Namespace: "address", Tag: "zip"},
		{Namespace: "address", Tag: "country"},
		{Namespace: "geo", Tag: "lat"},
		{Namespace: "geo", Tag: "lng"},
		{Namespace: "timezone"},
		{Namespace: "currency", Tag: "usd", Params: []string{"1", "2"}},
		{Namespace: "credit_card"},
		{Namespace: "credit_card", Tag: "type"},
		{Namespace: "iban"},
		{Namespace: "swift"},
		{Namespace: "text", Tag: "word"},
		{Namespace: "text", Tag: "sentence"},
		{Namespace: "text", Tag: "paragraph"},
		{Namespace: "text", Tag: "slug"},
		{Namespace: "product", Tag: "title"},
		{Namespace: "product", Tag: "description"},
		{Namespace: "product", Tag: "sku"},
		{Namespace: "company", Tag: "name"},
		{Namespace: "company", Tag: "industry"},
		{Namespace: "company", Tag: "catch_phrase"},
		{Namespace: "job", Tag: "title"},
		{Namespace: "job", Tag: "department"},
		{Namespace: "color", Tag: "hex"},
		{Namespace: "color", Tag: "rgb"},
		{Namespace: "color", Tag: "name"},
		{Namespace: "file", Tag: "name"},
		{Namespace: "file", Tag: "extension"},
		{Namespace: "file", Tag: "mime"},
		{Namespace: "hash", Tag: "md5"},
		{Namespace: "hash", Tag: "sha256"},
		{Namespace: "slug"},
		{Namespace: "code"},
		{Namespace: "unknown", Tag: "thing"},
	}
	for _, tag := range tags {
		got, err := eng.generateSemantic(tag, rng)
		if err != nil {
			t.Fatalf("%+v: %v", tag, err)
		}
		if got.Kind == value.KindNull {
			t.Fatalf("%+v produced null", tag)
		}
	}
}

func TestGenerateSemanticFallbacksWithoutCorpus(t *testing.T) {
	eng := New(nil)
	rng := NewRand(5)
	for _, tag := range []model.Semantic{
		{Namespace: "person", Tag: "username"},
		{Namespace: "person", Tag: "prefix"},
		{Namespace: "person", Tag: "suffix"},
		{Namespace: "person", Tag: "bio"},
		{Namespace: "address", Tag: "city"},
		{Namespace: "timezone"},
		{Namespace: "file", Tag: "extension"},
	} {
		got, err := eng.generateSemantic(tag, rng)
		if err != nil {
			t.Fatalf("%+v: %v", tag, err)
		}
		if strings.TrimSpace(valueDisplay(got)) == "" {
			t.Fatalf("%+v produced empty value", tag)
		}
	}
}
