package validate

type Alias struct {
	Type string `json:"type"`
}

type Item struct {
	Type      string        `json:"type"`
	Name      string        `json:"name"`
	HasMin    bool          `json:"hasMin"`
	HasMax    bool          `json:"hasMax"`
	Min       interface{}   `json:"min"`
	Max       interface{}   `json:"max"`
	Required  bool          `json:"required"`
	Enum      []interface{} `json:"enum"`
	MatchCase bool          `json:"case"`
}

type Object struct {
	Fields []Item `json:"fields"`
}

type Array struct {
	Min  int  `json:"min"`
	Max  int  `json:"max"`
	Type Item `json:"type"`
}

// Validation types that start with this are private to the validation
// package and cannot be used to create dictionary key elements.
const privateTypePrefix = "_"

const (
	AnyType      = privateTypePrefix + "any"
	IntType      = privateTypePrefix + "int"
	FloatType    = privateTypePrefix + "float"
	NumType      = privateTypePrefix + "num"
	StringType   = privateTypePrefix + "string"
	BoolType     = privateTypePrefix + "bool"
	DurationType = privateTypePrefix + "duration"
	TimeType     = privateTypePrefix + "time"
	UUIDType     = privateTypePrefix + "uuid"
	AliasType    = privateTypePrefix + "alias"
	ArrayType    = privateTypePrefix + "array"
	ObjectType   = privateTypePrefix + "object"
	ItemType     = privateTypePrefix + "element"
)
