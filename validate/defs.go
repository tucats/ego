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

const (
	AnyType      = "_any"
	IntType      = "_int"
	FloatType    = "_float"
	NumType      = "_num"
	StringType   = "_string"
	BoolType     = "_bool"
	DurationType = "_duration"
	TimeType     = "_time"
	UUIDType     = "_uuid"
	AliasType    = "_alias"
	ArrayType    = "_array"
	ObjectType   = "_object"
	ItemType     = "_element"
)
