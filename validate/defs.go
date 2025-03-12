package validate

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

const (
	AnyType      = "any"
	IntType      = "int"
	FloatType    = "float"
	NumType      = "num"
	StringType   = "string"
	BoolType     = "bool"
	DurationType = "duration"
	TimeType     = "time"
	UUIDType     = "uuid"
	ArrayType    = "array"
	ObjectType   = "object"
)
