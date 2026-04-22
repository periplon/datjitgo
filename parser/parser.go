// Package parser implements the ports.Parser adapter: it turns a DDL YAML
// document into a fully populated *model.Document while preserving the
// declaration order of entities, fields and enums. The parser is intentionally
// permissive about unfamiliar decorators and tool shapes — validation lives
// in core/validator in a later phase.
package parser

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	derrs "github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/model"
	"github.com/jmcarbo/datjitgo/core/ports"
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
