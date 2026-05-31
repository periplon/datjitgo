// Package parser contains the default datjit schema parser adapter.
//
// It implements the ports.Parser adapter by turning YAML schema input into a
// fully populated *model.Document while preserving declaration order for
// entities, fields, and enums. The parser is intentionally permissive about
// unfamiliar decorators and tool shapes so later validation can report domain
// errors.
package parser

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	derrs "github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
)

// Parser is the adapter implementation. It is stateless; one instance can be
// reused across calls.
type Parser struct{}

// New returns a ready-to-use Parser.
func New() *Parser { return &Parser{} }

// Ensure Parser satisfies the ports interface at compile time.
var _ ports.Parser = (*Parser)(nil)

// Parse decodes the YAML payload from r into a *model.Document. `name` is
// used for error Location.File.
func (p *Parser) Parse(r io.Reader, name string) (*model.Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, &derrs.Error{
			Kind:     derrs.KindParse,
			Location: &derrs.Location{File: name},
			Message:  fmt.Sprintf("reading input: %v", err),
			Cause:    err,
		}
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, &derrs.Error{
			Kind:     derrs.KindParse,
			Location: &derrs.Location{File: name},
			Message:  fmt.Sprintf("invalid YAML: %v", err),
			Cause:    err,
		}
	}

	return parseDocument(name, &root)
}
