// Package runtime exposes datjitgo generation as embeddable operations for
// DSLs, rule engines, and other host runtimes.
package runtime

import (
	"context"
	"strings"

	datjit "github.com/periplon/datjitgo"
	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

// Runtime is the embeddable generation API.
type Runtime interface {
	GenerateDocument(ctx context.Context, doc *model.Document, opts ...RunOption) (*value.Dataset, error)
	GenerateEntity(ctx context.Context, doc *model.Document, entity string, opts ...RunOption) ([]*value.Object, error)
	GenerateRows(ctx context.Context, req RowsRequest) ([]*value.Object, error)
	GenerateValue(ctx context.Context, req ValueRequest) (value.Value, error)
}

// RowsRequest describes a row generation request for one entity.
type RowsRequest struct {
	// Document is the parsed schema to generate from. Required.
	Document *model.Document
	// Entity is the name of the entity whose rows to generate. Required.
	Entity string
	// Seed, when non-nil, pins the generation seed for this call.
	Seed *int64
	// Locale, when non-empty, overrides the locale for this call.
	Locale string
	// Volumes overrides generated row counts per entity name; empty entries
	// are ignored and the document-declared volumes are used instead.
	Volumes map[string]int
}

// ValueRequest describes a single value generation request.
type ValueRequest struct {
	// Type is the field type to generate. Ignored when Semantic is set.
	Type model.TypeExpr
	// Semantic names a semantic type (e.g. "person.first"); when non-empty it
	// overrides Type. If both Type and Semantic are empty the value defaults to
	// the "any" primitive.
	Semantic string
	// Decorators are applied to the synthesized field (e.g. @range, @pattern).
	Decorators []model.Decorator
	// Seed, when non-nil, pins the generation seed for this call.
	Seed *int64
	// Locale, when non-empty, overrides the locale for this call.
	Locale string
	// UniqueKey names the synthesized field; it defaults to "value" when empty.
	UniqueKey string
}

// DocumentCompiler compiles host-language input into a datjit document.
type DocumentCompiler interface {
	Compile(ctx context.Context, src any) (*model.Document, error)
}

// CompileFunc adapts a function to DocumentCompiler.
type CompileFunc func(ctx context.Context, src any) (*model.Document, error)

// Compile implements DocumentCompiler.
func (f CompileFunc) Compile(ctx context.Context, src any) (*model.Document, error) {
	return f(ctx, src)
}

// Default is the service-backed Runtime implementation.
type Default struct {
	service *datjit.Service
}

var _ Runtime = (*Default)(nil)

// NewDefault returns a Runtime backed by datjit.NewDefault().
func NewDefault() *Default {
	return &Default{service: datjit.NewDefault()}
}

// New returns a Runtime backed by datjit.New(opts...).
func New(opts ...datjit.Option) (*Default, error) {
	svc, err := datjit.New(opts...)
	if err != nil {
		return nil, err
	}
	return &Default{service: svc}, nil
}

type runConfig struct {
	seed    *int64
	locale  string
	volumes map[string]int
	entity  string
}

// RunOption configures one generation call.
type RunOption func(*runConfig)

// WithSeed pins the seed for one runtime generation call.
func WithSeed(seed int64) RunOption {
	return func(c *runConfig) {
		v := seed
		c.seed = &v
	}
}

// WithLocale pins the locale for one runtime generation call.
func WithLocale(locale string) RunOption {
	return func(c *runConfig) {
		c.locale = locale
	}
}

// WithVolume overrides the generated row count for a single entity. Repeated
// calls accumulate; calling it for the same entity again overrides the prior
// value for that entity. It does not clear overrides for other entities.
func WithVolume(entity string, volume int) RunOption {
	return func(c *runConfig) {
		if c.volumes == nil {
			c.volumes = map[string]int{}
		}
		c.volumes[entity] = volume
	}
}

// WithVolumes overrides generated row counts for multiple entities at once. It
// merges the given entries into any overrides already set by earlier WithVolume
// or WithVolumes calls (later entries win for the same entity); an empty or nil
// map is a no-op.
func WithVolumes(volumes map[string]int) RunOption {
	return func(c *runConfig) {
		if len(volumes) == 0 {
			return
		}
		if c.volumes == nil {
			c.volumes = map[string]int{}
		}
		for entity, volume := range volumes {
			c.volumes[entity] = volume
		}
	}
}

// WithEntity filters the returned dataset to one entity.
func WithEntity(entity string) RunOption {
	return func(c *runConfig) {
		c.entity = entity
	}
}

// GenerateDocument validates and generates a full document, then applies any
// entity filter requested by RunOption.
func (r *Default) GenerateDocument(ctx context.Context, doc *model.Document, opts ...RunOption) (*value.Dataset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil || r.service == nil {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "runtime.Default.GenerateDocument: nil runtime"}
	}
	if doc == nil {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "nil document"}
	}
	cfg := applyRunOptions(opts)
	if cfg.entity != "" && !doc.Entities.Has(cfg.entity) {
		return nil, &errors.Error{Kind: errors.KindValidation, Message: "entity not found: " + cfg.entity}
	}

	runDoc := cloneDocument(doc)
	applyConfig(runDoc, cfg)
	if err := r.service.Validate(runDoc); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ds, err := r.service.Generate(runDoc)
	if err != nil {
		return nil, err
	}
	if cfg.entity != "" {
		return filterDataset(ds, cfg.entity), nil
	}
	return ds, nil
}

// GenerateEntity validates and generates rows for one entity.
func (r *Default) GenerateEntity(ctx context.Context, doc *model.Document, entity string, opts ...RunOption) ([]*value.Object, error) {
	opts = append(append([]RunOption(nil), opts...), WithEntity(entity))
	ds, err := r.GenerateDocument(ctx, doc, opts...)
	if err != nil {
		return nil, err
	}
	rows, _ := ds.Entities.Get(entity)
	return rows, nil
}

// GenerateRows validates and generates rows for the entity named in req.
func (r *Default) GenerateRows(ctx context.Context, req RowsRequest) ([]*value.Object, error) {
	opts := []RunOption{}
	if req.Seed != nil {
		opts = append(opts, WithSeed(*req.Seed))
	}
	if req.Locale != "" {
		opts = append(opts, WithLocale(req.Locale))
	}
	if len(req.Volumes) > 0 {
		opts = append(opts, WithVolumes(req.Volumes))
	}
	return r.GenerateEntity(ctx, req.Document, req.Entity, opts...)
}

// GenerateValue compiles a single-field temporary document and returns the
// generated field value.
func (r *Default) GenerateValue(ctx context.Context, req ValueRequest) (value.Value, error) {
	if err := ctx.Err(); err != nil {
		return value.Null(), err
	}
	fieldName := req.UniqueKey
	if fieldName == "" {
		fieldName = "value"
	}
	typ := req.Type
	if req.Semantic != "" {
		typ = semanticType(req.Semantic)
	}
	if typ == nil {
		typ = model.Primitive{Kind: model.PrimAny}
	}

	doc := model.NewDocument()
	ent := model.NewEntity("value_request")
	ent.Fields.Set(fieldName, &model.Field{
		Name:       fieldName,
		Type:       typ,
		Decorators: cloneDecorators(req.Decorators),
	})
	doc.Entities.Set(ent.Name, ent)
	doc.Volume[ent.Name] = model.VolumeSpec{Exact: 1}

	opts := []RunOption{WithEntity(ent.Name)}
	if req.Seed != nil {
		opts = append(opts, WithSeed(*req.Seed))
	}
	if req.Locale != "" {
		opts = append(opts, WithLocale(req.Locale))
	}
	rows, err := r.GenerateEntity(ctx, doc, ent.Name, opts...)
	if err != nil {
		return value.Null(), err
	}
	if len(rows) == 0 {
		return value.Null(), &errors.Error{Kind: errors.KindGeneration, Message: "no value generated"}
	}
	got, ok := rows[0].Get(fieldName)
	if !ok {
		return value.Null(), &errors.Error{Kind: errors.KindGeneration, Message: "generated row missing value"}
	}
	return got, nil
}

func applyRunOptions(opts []RunOption) runConfig {
	var cfg runConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func applyConfig(doc *model.Document, cfg runConfig) {
	if cfg.seed != nil {
		v := *cfg.seed
		doc.Generation.Seed = &v
	}
	if cfg.locale != "" {
		doc.Generation.Locale = cfg.locale
	}
	for entity, volume := range cfg.volumes {
		doc.Volume[entity] = model.VolumeSpec{Exact: volume}
	}
}

func semanticType(name string) model.Semantic {
	namespace, tag, ok := strings.Cut(name, ".")
	if !ok {
		return model.Semantic{Namespace: name}
	}
	return model.Semantic{Namespace: namespace, Tag: tag}
}

func filterDataset(ds *value.Dataset, entity string) *value.Dataset {
	out := value.NewDataset()
	if ds == nil || ds.Entities == nil {
		return out
	}
	if rows, ok := ds.Entities.Get(entity); ok {
		out.Entities.Set(entity, rows)
	}
	return out
}

func cloneDocument(doc *model.Document) *model.Document {
	out := model.NewDocument()
	out.Domain = doc.Domain
	out.Version = doc.Version
	out.Seed = cloneInt64Ptr(doc.Seed)
	out.Locale = doc.Locale
	out.Generation = doc.Generation
	out.Generation.Seed = cloneInt64Ptr(doc.Generation.Seed)
	out.Volume = make(map[string]model.VolumeSpec, len(doc.Volume))
	for k, v := range doc.Volume {
		out.Volume[k] = v
	}
	out.Rules = append([]model.Rule(nil), doc.Rules...)
	out.Tools = make(map[string]model.ToolOverride, len(doc.Tools))
	for k, v := range doc.Tools {
		out.Tools[k] = v
	}
	doc.Entities.Each(func(name string, ent *model.Entity) bool {
		out.Entities.Set(name, cloneEntity(ent))
		return true
	})
	doc.Enums.Each(func(name string, def model.EnumDef) bool {
		def.Variants = append([]model.EnumVariant(nil), def.Variants...)
		out.Enums.Set(name, def)
		return true
	})
	doc.Types.Each(func(name string, ent *model.Entity) bool {
		out.Types.Set(name, cloneEntity(ent))
		return true
	})
	return out
}

func cloneEntity(ent *model.Entity) *model.Entity {
	if ent == nil {
		return nil
	}
	out := model.NewEntity(ent.Name)
	out.Meta = cloneDecorators(ent.Meta)
	ent.Fields.Each(func(name string, field *model.Field) bool {
		out.Fields.Set(name, cloneField(field))
		return true
	})
	ent.Coherence.Each(func(name string, fields []string) bool {
		out.Coherence.Set(name, append([]string(nil), fields...))
		return true
	})
	return out
}

func cloneField(field *model.Field) *model.Field {
	if field == nil {
		return nil
	}
	out := *field
	out.Decorators = cloneDecorators(field.Decorators)
	out.Compute = append([]model.ComputeBranch(nil), field.Compute...)
	if field.DefaultChain != nil {
		chain := *field.DefaultChain
		chain.Sources = append([]string(nil), field.DefaultChain.Sources...)
		out.DefaultChain = &chain
	}
	return &out
}

func cloneDecorators(in []model.Decorator) []model.Decorator {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.Decorator, len(in))
	for i, dec := range in {
		out[i] = dec
		out[i].Args = append([]model.DecoratorArg(nil), dec.Args...)
	}
	return out
}

func cloneInt64Ptr(in *int64) *int64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
