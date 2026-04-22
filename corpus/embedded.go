package corpus

import "embed"

// embedded holds the JSON corpus files shipped with the binary. Each file
// corresponds to one corpus key, with dots in the key replaced by underscores
// (e.g. "person.first_names" -> "data/person_first_names.json"). See
// provider.go for the loader.
//
//go:embed data/*.json
var embedded embed.FS
