package output

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/periplon/datjitgo/core/model"
	"github.com/periplon/datjitgo/core/value"
)

// NewFixture builds a deterministic (Document, Dataset) pair used across
// writer tests. Two entities are defined — User then Order — covering every
// value.Kind at least once.
func NewFixture(t *testing.T) (*model.Document, *value.Dataset) {
	t.Helper()

	doc := model.NewDocument()

	// --- User entity ------------------------------------------------------
	user := model.NewEntity("User")
	user.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimUUID}})
	user.Fields.Set("name", &model.Field{Name: "name", Type: model.Primitive{Kind: model.PrimString}})
	user.Fields.Set("age", &model.Field{Name: "age", Type: model.Primitive{Kind: model.PrimInt}})
	user.Fields.Set("score", &model.Field{Name: "score", Type: model.Primitive{Kind: model.PrimFloat}})
	user.Fields.Set("active", &model.Field{Name: "active", Type: model.Primitive{Kind: model.PrimBool}})
	user.Fields.Set("created_at", &model.Field{Name: "created_at", Type: model.Primitive{Kind: model.PrimDatetime}})
	user.Fields.Set("balance", &model.Field{Name: "balance", Type: model.Primitive{Kind: model.PrimDecimal, Params: []int{12, 2}}})
	user.Fields.Set("tags", &model.Field{Name: "tags", Type: model.List{Element: model.Primitive{Kind: model.PrimString}}})
	user.Fields.Set("meta", &model.Field{Name: "meta", Type: model.Map{Key: model.Primitive{Kind: model.PrimString}, Value: model.Primitive{Kind: model.PrimString}}})
	user.Fields.Set("nickname", &model.Field{Name: "nickname", Type: model.Nullable{Inner: model.Primitive{Kind: model.PrimString}}})
	doc.Entities.Set("User", user)

	// --- Order entity -----------------------------------------------------
	order := model.NewEntity("Order")
	order.Fields.Set("id", &model.Field{Name: "id", Type: model.Primitive{Kind: model.PrimUUID}})
	order.Fields.Set("user_id", &model.Field{Name: "user_id", Type: model.Primitive{Kind: model.PrimUUID}})
	order.Fields.Set("total", &model.Field{Name: "total", Type: model.Primitive{Kind: model.PrimDecimal, Params: []int{10, 2}}})
	doc.Entities.Set("Order", order)

	// --- Dataset ----------------------------------------------------------
	ds := value.NewDataset()

	u1 := value.NewObject()
	u1.Set("id", value.UUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")))
	u1.Set("name", value.Str("Alice"))
	u1.Set("age", value.Int(30))
	u1.Set("score", value.Float(3.5))
	u1.Set("active", value.Bool(true))
	u1.Set("created_at", value.Time(time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)))
	u1.Set("balance", value.Dec(decimal.RequireFromString("1234.56")))
	u1.Set("tags", value.List([]value.Value{value.Str("admin"), value.Str("early")}))
	metaObj := value.NewObject()
	metaObj.Set("team", value.Str("platform"))
	u1.Set("meta", value.Obj(metaObj))
	u1.Set("nickname", value.Str("Al"))

	u2 := value.NewObject()
	u2.Set("id", value.UUID(uuid.MustParse("22222222-2222-2222-2222-222222222222")))
	u2.Set("name", value.Str("Bob O'Brien"))
	u2.Set("age", value.Int(25))
	u2.Set("score", value.Float(1.25))
	u2.Set("active", value.Bool(false))
	u2.Set("created_at", value.Time(time.Date(2026, 4, 22, 13, 0, 0, 0, time.UTC)))
	u2.Set("balance", value.Dec(decimal.RequireFromString("0.00")))
	u2.Set("tags", value.List([]value.Value{}))
	u2.Set("meta", value.Obj(value.NewObject()))
	u2.Set("nickname", value.Null())

	ds.Entities.Set("User", []*value.Object{u1, u2})

	o1 := value.NewObject()
	o1.Set("id", value.UUID(uuid.MustParse("33333333-3333-3333-3333-333333333333")))
	o1.Set("user_id", value.UUID(uuid.MustParse("11111111-1111-1111-1111-111111111111")))
	o1.Set("total", value.Dec(decimal.RequireFromString("99.99")))

	ds.Entities.Set("Order", []*value.Object{o1})

	return doc, ds
}
