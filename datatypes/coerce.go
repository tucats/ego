package datatypes

func (t Type) Coerce(v interface{}) interface{} {
	switch t.kind {
	case intKind:
		return GetInt(v)

	case floatKind:
		return GetFloat(v)

	case stringKind:
		return GetString(v)

	case boolKind:
		return GetBool(v)
	}

	return v
}
