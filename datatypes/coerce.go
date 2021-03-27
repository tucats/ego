package datatypes

func (t Type) Coerce(v interface{}) interface{} {
	switch t.kind {
	case IntKind:
		return GetInt(v)

	case FloatKind:
		return GetFloat(v)

	case StringKind:
		return GetString(v)

	case BoolKind:
		return GetBool(v)
	}

	return v
}
