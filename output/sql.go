package output

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/periplon/datjitgo/core/errors"
	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/ports"
	"github.com/periplon/datjitgo/core/value"
)

const sqlBatchSize = 100

// SQL is the SQL output writer. Emits a CREATE TABLE followed by batched
// INSERT statements for each entity. The dialect is chosen via
// opts.SQLDialect (postgres|mysql|sqlite), defaulting to postgres.
type SQL struct{}

// NewSQL returns a new SQL writer.
func NewSQL() *SQL { return &SQL{} }

// Format returns "sql".
func (*SQL) Format() string { return "sql" }

// Write serialises ds to w.
func (s *SQL) Write(ds *value.Dataset, doc *model.Document, w io.Writer, opts ports.WriteOptions) error {
	if doc == nil {
		return &errors.Error{Kind: errors.KindValidation, Message: "sql writer requires Document"}
	}
	if ds == nil {
		return nil
	}
	dialect := strings.ToLower(opts.SQLDialect)
	switch dialect {
	case "":
		dialect = "postgres"
	case "postgres", "mysql", "sqlite":
	default:
		return &errors.Error{Kind: errors.KindValidation, Message: fmt.Sprintf("sql writer: unknown dialect %q", opts.SQLDialect)}
	}

	order := entityOrder(ds, doc, opts.EntityFilter)

	var buf bytes.Buffer
	for i, name := range order {
		if i > 0 {
			buf.WriteByte('\n')
		}
		ent, _ := doc.Entities.Get(name)
		rows, _ := ds.Entities.Get(name)
		fields := fieldOrder(rows, doc, name)

		if err := writeCreateTable(&buf, ent, name, fields, dialect); err != nil {
			return err
		}
		if len(rows) == 0 {
			continue
		}
		if err := writeInserts(&buf, name, fields, rows, dialect); err != nil {
			return err
		}
	}
	return writeAll(w, buf.Bytes())
}

func writeCreateTable(buf *bytes.Buffer, ent *model.Entity, name string, fields []string, dialect string) error {
	buf.WriteString("CREATE TABLE ")
	buf.WriteString(quoteSQLIdent(name, dialect))
	buf.WriteString(" (\n")
	for i, f := range fields {
		sep := ","
		if i == len(fields)-1 {
			sep = ""
		}
		typ := "TEXT"
		if ent != nil {
			if field, ok := ent.Fields.Get(f); ok {
				typ = sqlTypeFor(field.Type, dialect)
			}
		}
		fmt.Fprintf(buf, "  %s %s%s\n", quoteSQLIdent(f, dialect), typ, sep)
	}
	buf.WriteString(");\n")
	return nil
}

func writeInserts(buf *bytes.Buffer, name string, fields []string, rows []*value.Object, dialect string) error {
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = quoteSQLIdent(f, dialect)
	}
	colList := strings.Join(cols, ", ")
	table := quoteSQLIdent(name, dialect)

	for start := 0; start < len(rows); start += sqlBatchSize {
		end := start + sqlBatchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[start:end]

		fmt.Fprintf(buf, "INSERT INTO %s (%s) VALUES\n", table, colList)
		for i, row := range batch {
			values := make([]string, len(fields))
			for j, f := range fields {
				if row == nil {
					values[j] = "NULL"
					continue
				}
				v, ok := row.Get(f)
				if !ok {
					values[j] = "NULL"
					continue
				}
				lit, err := sqlLiteral(v, dialect)
				if err != nil {
					return wrapIO(err, "sql %s.%s", name, f)
				}
				values[j] = lit
			}
			term := ","
			if i == len(batch)-1 {
				term = ";"
			}
			fmt.Fprintf(buf, "  (%s)%s\n", strings.Join(values, ", "), term)
		}
	}
	return nil
}

func sqlLiteral(v value.Value, dialect string) (string, error) {
	switch v.Kind {
	case value.KindNull:
		return "NULL", nil
	case value.KindBool:
		if dialect == "postgres" {
			if v.B {
				return "TRUE", nil
			}
			return "FALSE", nil
		}
		// mysql, sqlite → 1/0
		if v.B {
			return "1", nil
		}
		return "0", nil
	case value.KindInt:
		return fmt.Sprintf("%d", v.I), nil
	case value.KindFloat:
		return formatFloat(v.F), nil
	case value.KindString:
		return escapeSQLString(v.S), nil
	case value.KindUUID:
		return escapeSQLString(v.U.String()), nil
	case value.KindTime:
		return escapeSQLString(v.T.UTC().Format(time.RFC3339Nano)), nil
	case value.KindDecimal:
		return v.D.String(), nil
	case value.KindList, value.KindObject:
		s, err := renderValueScalar(v)
		if err != nil {
			return "", err
		}
		return escapeSQLString(s), nil
	default:
		return "", fmt.Errorf("unknown value kind %d", v.Kind)
	}
}

// sqlTypeFor returns the SQL column type for a model TypeExpr in the given
// dialect.
func sqlTypeFor(t model.TypeExpr, dialect string) string {
	switch tt := t.(type) {
	case model.Nullable:
		return sqlTypeFor(tt.Inner, dialect)
	case model.Primitive:
		return primitiveSQLType(tt, dialect)
	case model.Semantic:
		// Semantic tags map to a string-like column unless they explicitly
		// represent a known primitive. UUID as a semantic tag is handled.
		if tt.Namespace == "uuid" && tt.Tag == "" {
			return uuidSQLType(dialect)
		}
		return stringSQLType(dialect)
	case model.EnumInline, model.NamedType:
		return stringSQLType(dialect)
	case model.Reference:
		// References serialise as the target's primary-key type; pick TEXT
		// generically since we don't yet walk the foreign entity in phase 1.
		return stringSQLType(dialect)
	case model.List, model.Map, model.Tuple:
		if dialect == "postgres" {
			return "JSONB"
		}
		return "TEXT"
	case model.Union:
		return stringSQLType(dialect)
	}
	return stringSQLType(dialect)
}

func primitiveSQLType(p model.Primitive, dialect string) string {
	switch p.Kind {
	case model.PrimString:
		return stringSQLType(dialect)
	case model.PrimInt:
		switch dialect {
		case "sqlite":
			return "INTEGER"
		default:
			return "BIGINT"
		}
	case model.PrimFloat:
		switch dialect {
		case "postgres":
			return "DOUBLE PRECISION"
		case "mysql":
			return "DOUBLE"
		case "sqlite":
			return "REAL"
		}
	case model.PrimBool:
		switch dialect {
		case "postgres":
			return "BOOLEAN"
		case "mysql":
			return "TINYINT(1)"
		case "sqlite":
			return "INTEGER"
		}
	case model.PrimDatetime:
		switch dialect {
		case "postgres":
			return "TIMESTAMPTZ"
		case "mysql":
			return "DATETIME"
		case "sqlite":
			return "TEXT"
		}
	case model.PrimDate:
		if dialect == "sqlite" {
			return "TEXT"
		}
		return "DATE"
	case model.PrimTime:
		if dialect == "sqlite" {
			return "TEXT"
		}
		return "TIME"
	case model.PrimDuration:
		return stringSQLType(dialect)
	case model.PrimUUID:
		return uuidSQLType(dialect)
	case model.PrimBytes:
		switch dialect {
		case "postgres":
			return "BYTEA"
		default:
			return "BLOB"
		}
	case model.PrimDecimal:
		precision, scale := 18, 2
		if len(p.Params) >= 1 {
			precision = p.Params[0]
		}
		if len(p.Params) >= 2 {
			scale = p.Params[1]
		}
		switch dialect {
		case "postgres":
			return fmt.Sprintf("NUMERIC(%d,%d)", precision, scale)
		case "mysql":
			return fmt.Sprintf("DECIMAL(%d,%d)", precision, scale)
		case "sqlite":
			return "REAL"
		}
	case model.PrimNull:
		return stringSQLType(dialect)
	case model.PrimAny:
		if dialect == "postgres" {
			return "JSONB"
		}
		return "TEXT"
	}
	return stringSQLType(dialect)
}

func stringSQLType(dialect string) string {
	// All three supported dialects use TEXT for free-form strings.
	_ = dialect
	return "TEXT"
}

func uuidSQLType(dialect string) string {
	switch dialect {
	case "postgres":
		return "UUID"
	default:
		return "CHAR(36)"
	}
}
