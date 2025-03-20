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
	AnyType      = ".ego.any"
	IntType      = ".ego.int"
	FloatType    = ".ego.float"
	NumType      = ".ego.num"
	StringType   = ".ego.string"
	BoolType     = ".ego.bool"
	DurationType = ".ego.duration"
	TimeType     = ".ego.time"
	UUIDType     = ".ego.uuid"
	ArrayType    = ",ego.array"
	ObjectType   = ".ego.object"
	ItemType     = ".ego.item"
)
