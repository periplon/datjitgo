package model

// EnumVariant is one value in an enum, optionally with display metadata.
type EnumVariant struct {
	Value       string
	Label       string
	Weight      *float64
	Description string
}

// EnumDef is a named enum declared under the top-level `enums:` section.
type EnumDef struct {
	Name     string
	Variants []EnumVariant
}

// Values returns the raw variant values in order.
func (e EnumDef) Values() []string {
	out := make([]string, len(e.Variants))
	for i, v := range e.Variants {
		out[i] = v.Value
	}
	return out
}

// WeightsOrNil returns variant weights if any variant carries one, else nil.
func (e EnumDef) WeightsOrNil() []float64 {
	anyWeighted := false
	for _, v := range e.Variants {
		if v.Weight != nil {
			anyWeighted = true
			break
		}
	}
	if !anyWeighted {
		return nil
	}
	out := make([]float64, len(e.Variants))
	for i, v := range e.Variants {
		if v.Weight != nil {
			out[i] = *v.Weight
		} else {
			out[i] = 1
		}
	}
	return out
}
