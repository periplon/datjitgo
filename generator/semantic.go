package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
	"github.com/jmcarbo/datjitgo/core/value"
)

// generateSemantic produces a value for a model.Semantic type expression.
//
// The dispatch first consults the CorpusProvider for a direct match on the
// fully-qualified tag (so custom corpora can override everything). When no
// corpus key exists, we fall back to a small set of synthesisers that
// construct composite values (email, phone, IP, hashes, colour codes, etc.).
func (e *Engine) generateSemantic(st model.Semantic, rng ports.Randomizer) (value.Value, error) {
	full := semanticFullName(st)

	// 1. Direct corpus lookup.
	if e.corpus != nil && e.corpus.Has(full) {
		return e.corpus.Sample(ports.SampleContext{Locale: e.locale, RNG: rng}, full)
	}

	// 2. Composite semantic types.
	switch full {
	case "person.full":
		first, err := e.sampleCorpusString(rng, "person.first_names")
		if err != nil {
			return value.Null(), err
		}
		last, err := e.sampleCorpusString(rng, "person.last_names")
		if err != nil {
			return value.Null(), err
		}
		return value.Str(first + " " + last), nil

	case "person.first":
		s, err := e.sampleCorpusString(rng, "person.first_names")
		if err != nil {
			return value.Null(), err
		}
		return value.Str(s), nil

	case "person.last":
		s, err := e.sampleCorpusString(rng, "person.last_names")
		if err != nil {
			return value.Null(), err
		}
		return value.Str(s), nil

	case "person.username":
		s, err := e.sampleCorpusString(rng, "person.usernames")
		if err != nil {
			return value.Str(fmt.Sprintf("user%d", rng.IntN(9900)+100)), nil
		}
		return value.Str(s), nil

	case "person.prefix":
		s, err := e.sampleCorpusString(rng, "person.prefixes")
		if err != nil {
			return value.Str("Mr."), nil
		}
		return value.Str(s), nil

	case "person.suffix":
		s, err := e.sampleCorpusString(rng, "person.suffixes")
		if err != nil {
			return value.Str("Jr."), nil
		}
		return value.Str(s), nil

	case "person.bio":
		s, err := e.sampleCorpusString(rng, "person.bios")
		if err != nil {
			return value.Str(fmt.Sprintf("Experienced professional with %d+ years in the industry.", rng.IntN(29)+1)), nil
		}
		return value.Str(s), nil

	case "person.avatar":
		return value.Str(fmt.Sprintf("https://i.pravatar.cc/150?u=%d", rng.IntN(9999)+1)), nil

	case "person.gender":
		genders := []string{"female", "male", "nonbinary"}
		return value.Str(genders[rng.IntN(int64(len(genders)))]), nil

	case "person.dob":
		year := 1950 + rng.IntN(55)
		month := rng.IntN(12) + 1
		day := rng.IntN(28) + 1
		return value.Str(fmt.Sprintf("%04d-%02d-%02d", year, month, day)), nil

	case "person.age":
		return value.Int(18 + rng.IntN(67)), nil

	case "email":
		first, _ := e.sampleCorpusString(rng, "person.first_names")
		last, _ := e.sampleCorpusString(rng, "person.last_names")
		domain, err := e.sampleCorpusString(rng, "email.domains")
		if err != nil {
			domain = "example.com"
		}
		addr := fmt.Sprintf("%s.%s@%s", strings.ToLower(first), strings.ToLower(last), domain)
		return value.Str(strings.ReplaceAll(addr, " ", "")), nil

	case "phone", "phone.mobile", "phone.landline":
		area, err := e.sampleCorpusString(rng, "phone.area_codes")
		if err != nil || area == "" {
			area = fmt.Sprintf("%03d", 200+rng.IntN(799))
		}
		mid := rng.IntN(900) + 100
		tail := rng.IntN(9000) + 1000
		return value.Str(fmt.Sprintf("+1-%s-%03d-%04d", area, mid, tail)), nil

	case "url":
		return value.Str(fmt.Sprintf("https://example.com/page/%d", rng.IntN(10000))), nil
	case "url.image":
		return value.Str(fmt.Sprintf("https://picsum.photos/400/300?id=%d", rng.IntN(1000)+1)), nil
	case "url.avatar":
		return value.Str(fmt.Sprintf("https://i.pravatar.cc/150?u=%d", rng.IntN(10000)+1)), nil

	case "ipv4":
		return value.Str(fmt.Sprintf("%d.%d.%d.%d",
			rng.IntN(254)+1, rng.IntN(256), rng.IntN(256), rng.IntN(254)+1)), nil
	case "ipv6":
		parts := make([]string, 8)
		for i := range parts {
			parts[i] = fmt.Sprintf("%04x", rng.IntN(0x10000))
		}
		return value.Str(strings.Join(parts, ":")), nil
	case "mac":
		parts := make([]string, 6)
		for i := range parts {
			parts[i] = fmt.Sprintf("%02X", rng.IntN(256))
		}
		return value.Str(strings.Join(parts, ":")), nil

	case "address.full":
		city, _ := e.sampleCorpusString(rng, "address.cities")
		state, _ := e.sampleCorpusString(rng, "address.states")
		street, _ := e.sampleCorpusString(rng, "address.streets")
		num := rng.IntN(9900) + 100
		zip := rng.IntN(90000) + 10000
		return value.Str(fmt.Sprintf("%d %s, %s, %s %05d", num, street, city, state, zip)), nil
	case "address.street":
		street, _ := e.sampleCorpusString(rng, "address.streets")
		num := rng.IntN(9900) + 100
		return value.Str(fmt.Sprintf("%d %s", num, street)), nil
	case "address.city":
		s, err := e.sampleCorpusString(rng, "address.cities")
		if err != nil {
			return value.Str("Springfield"), nil
		}
		return value.Str(s), nil
	case "address.state":
		s, err := e.sampleCorpusString(rng, "address.states")
		if err != nil {
			return value.Str("CA"), nil
		}
		return value.Str(s), nil
	case "address.zip":
		return value.Str(fmt.Sprintf("%05d", rng.IntN(90000)+10000)), nil
	case "address.country":
		s, err := e.sampleCorpusString(rng, "address.countries")
		if err != nil {
			return value.Str("US"), nil
		}
		return value.Str(s), nil

	case "geo.lat":
		return value.Float(roundTo(-90+rng.Float()*180, 4)), nil
	case "geo.lng":
		return value.Float(roundTo(-180+rng.Float()*360, 4)), nil

	case "timezone":
		s, err := e.sampleCorpusString(rng, "timezones")
		if err != nil {
			return value.Str("America/New_York"), nil
		}
		return value.Str(s), nil

	case "currency.usd", "currency.eur":
		lo, hi := 1.0, 1000.0
		if len(st.Params) >= 2 {
			if parsedLo, err := strconv.ParseFloat(strings.TrimSpace(st.Params[0]), 64); err == nil {
				if parsedHi, err := strconv.ParseFloat(strings.TrimSpace(st.Params[1]), 64); err == nil && parsedHi >= parsedLo {
					lo, hi = parsedLo, parsedHi
				}
			}
		}
		return value.Float(roundTo(lo+rng.Float()*(hi-lo), 2)), nil

	case "credit_card":
		return value.Str(fmt.Sprintf("4111-%04d-%04d-%04d",
			rng.IntN(9000)+1000, rng.IntN(9000)+1000, rng.IntN(9000)+1000)), nil
	case "credit_card.type":
		types := []string{"visa", "mastercard", "amex", "discover"}
		return value.Str(types[rng.IntN(int64(len(types)))]), nil
	case "iban":
		return value.Str(fmt.Sprintf("DE%02d%04d%04d%04d%04d%02d",
			rng.IntN(90)+10, rng.IntN(9000)+1000, rng.IntN(9000)+1000,
			rng.IntN(9000)+1000, rng.IntN(9000)+1000, rng.IntN(90)+10)), nil
	case "swift":
		return value.Str("COBADEFFXXX"), nil

	case "text.word":
		s, err := e.sampleCorpusString(rng, "text.words")
		if err != nil {
			return value.Str("word"), nil
		}
		return value.Str(s), nil
	case "text.sentence":
		s, err := e.sampleCorpusString(rng, "text.sentences")
		if err != nil {
			return value.Str("The quick brown fox jumps over the lazy dog."), nil
		}
		return value.Str(s), nil
	case "text.paragraph":
		s, err := e.sampleCorpusString(rng, "text.paragraphs")
		if err != nil {
			return value.Str("Lorem ipsum dolor sit amet."), nil
		}
		return value.Str(s), nil
	case "text.slug":
		w1, _ := e.sampleCorpusString(rng, "text.words")
		w2, _ := e.sampleCorpusString(rng, "text.words")
		if w1 == "" {
			w1 = "new"
		}
		if w2 == "" {
			w2 = "item"
		}
		return value.Str(fmt.Sprintf("%s-%s-%d", strings.ToLower(w1), strings.ToLower(w2), rng.IntN(999)+1)), nil

	case "product.title":
		s, err := e.sampleCorpusString(rng, "product.titles")
		if err != nil {
			return value.Str("Generic Item"), nil
		}
		return value.Str(s), nil
	case "product.description":
		s, err := e.sampleCorpusString(rng, "product.descriptions")
		if err != nil {
			return value.Str("High-quality product."), nil
		}
		return value.Str(s), nil
	case "product.sku", "sku":
		return value.Str(fmt.Sprintf("SKU-%c%c-%04d",
			byte('A'+rng.IntN(26)), byte('A'+rng.IntN(26)), rng.IntN(9999)+1)), nil

	case "company.name":
		s, err := e.sampleCorpusString(rng, "company.names")
		if err != nil {
			return value.Str("Acme Inc."), nil
		}
		return value.Str(s), nil
	case "company.industry":
		s, err := e.sampleCorpusString(rng, "company.industries")
		if err != nil {
			return value.Str("Technology"), nil
		}
		return value.Str(s), nil
	case "company.catch_phrase":
		s, err := e.sampleCorpusString(rng, "company.catch_phrases")
		if err != nil {
			return value.Str("Innovate. Integrate. Excel."), nil
		}
		return value.Str(s), nil

	case "job.title":
		s, err := e.sampleCorpusString(rng, "job.titles")
		if err != nil {
			return value.Str("Engineer"), nil
		}
		return value.Str(s), nil
	case "job.department":
		s, err := e.sampleCorpusString(rng, "job.departments")
		if err != nil {
			return value.Str("Engineering"), nil
		}
		return value.Str(s), nil

	case "color.hex":
		return value.Str(fmt.Sprintf("#%06x", rng.IntN(0x1000000))), nil
	case "color.rgb":
		return value.Str(fmt.Sprintf("rgb(%d, %d, %d)",
			rng.IntN(256), rng.IntN(256), rng.IntN(256))), nil
	case "color.name":
		s, err := e.sampleCorpusString(rng, "color.names")
		if err != nil {
			return value.Str("coral"), nil
		}
		return value.Str(s), nil

	case "file.name":
		base := []string{"report", "document", "image", "data", "backup"}
		ext, _ := e.sampleCorpusString(rng, "file.extensions")
		if ext == "" {
			ext = "bin"
		}
		return value.Str(fmt.Sprintf("%s_%d.%s", base[rng.IntN(int64(len(base)))], rng.IntN(999)+1, strings.TrimPrefix(ext, "."))), nil
	case "file.extension":
		s, err := e.sampleCorpusString(rng, "file.extensions")
		if err != nil {
			return value.Str(".pdf"), nil
		}
		if !strings.HasPrefix(s, ".") {
			s = "." + s
		}
		return value.Str(s), nil
	case "file.mime":
		s, err := e.sampleCorpusString(rng, "mime.types")
		if err != nil {
			return value.Str("application/octet-stream"), nil
		}
		return value.Str(s), nil

	case "hash.md5":
		return value.Str(randomHex(rng, 32)), nil
	case "hash.sha256":
		return value.Str(randomHex(rng, 64)), nil

	case "slug":
		return value.Str(fmt.Sprintf("item-%d", rng.IntN(9000)+1000)), nil
	case "code":
		return value.Str(fmt.Sprintf("%c%c%c%03d",
			byte('A'+rng.IntN(26)), byte('A'+rng.IntN(26)),
			byte('A'+rng.IntN(26)), rng.IntN(999)+1)), nil
	}

	// Unknown tag — keep output valid by synthesising a stable placeholder.
	return value.Str(fmt.Sprintf("%s_%d", full, rng.IntN(9999)+1)), nil
}

// sampleCorpusString samples a string value from the corpus; nil provider or
// missing key yields an error so callers can apply their fallback.
func (e *Engine) sampleCorpusString(rng ports.Randomizer, key string) (string, error) {
	if e.corpus == nil || !e.corpus.Has(key) {
		return "", &errors.Error{Kind: errors.KindCorpusMissing, Message: key}
	}
	v, err := e.corpus.Sample(ports.SampleContext{Locale: e.locale, RNG: rng}, key)
	if err != nil {
		return "", err
	}
	if v.Kind == value.KindString {
		return v.S, nil
	}
	return "", &errors.Error{Kind: errors.KindCorpusMissing, Message: "non-string corpus entry for " + key}
}

// semanticFullName returns the dotted tag for a Semantic type expression.
func semanticFullName(st model.Semantic) string {
	if st.Tag == "" {
		return st.Namespace
	}
	return st.Namespace + "." + st.Tag
}

func randomHex(rng ports.Randomizer, n int) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = hex[rng.IntN(16)]
	}
	return string(buf)
}

func roundTo(f float64, places int) float64 {
	factor := 1.0
	for i := 0; i < places; i++ {
		factor *= 10
	}
	r := f * factor
	if r < 0 {
		r -= 0.5
	} else {
		r += 0.5
	}
	return float64(int64(r)) / factor
}
