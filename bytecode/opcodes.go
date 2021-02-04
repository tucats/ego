package bytecode

/*
 * ADDING A NEW OPCODE
 *
 * 1. Add the Opcode name as a constant in the list below. If it is an opcode
 *    that has a bytecode address as its operand, put it in the section
 *    identified as "branch instructions".
 *
 * 2. Add the opcode name to the map below, which converts the const identifier
 *    to a human-readable name. By convention, the human-readable name is the same as
 *    the constant itself.
 *
 * 3. Add the dispatch entry which points to the function that implements the opcode.
 *
 * 4. Implement the actual opcode, nominally in the appropriate op_*.go file.
 */

// Constant describing instruction opcodes

type Instruction int

const (
	Stop   Instruction = 0
	AtLine             = iota + BuiltinInstructions
	Add    Instruction = iota
	And
	ArgCheck
	Array
	Auth
	Call
	ClassMember
	Coerce
	Constant
	Copy
	Div
	Drop
	DropToMarker
	Dup
	Equal
	Exp
	Flatten
	FromFile
	GetVarArgs
	Go
	GreaterThan
	GreaterThanOrEqual
	LessThan
	LessThanOrEqual
	Load
	LoadIndex
	LoadSlice
	Log
	MakeArray
	Member
	Mul
	Negate
	Newline
	NotEqual
	Or
	Panic
	PopScope
	Print
	Push
	PushScope
	RangeInit
	RangeNext
	RequiredType
	Response
	Return
	Say
	StackCheck
	StaticTyping
	Store
	StoreAlways
	StoreChan
	StoreGlobal
	StoreIndex
	StoreInto
	StoreMetadata
	Struct
	Sub
	Swap
	SymbolCreate
	SymbolDelete
	SymbolOptCreate
	Template
	This
	Try
	TryPop

	// Everything from here on is a branch instruction, whose
	// operand must be present and is an integer instruction
	// address in the bytecode array.
	BranchInstructions = iota + BranchInstruction
	Branch
	BranchTrue
	BranchFalse
	LocalCall

	// After this value, additional user branch instructions are
	// can be defined.
	UserBranchInstructions
)

var instructionNames = map[Instruction]string{
	Add:                "Add",
	And:                "And",
	ArgCheck:           "ArgCheck",
	Array:              "Array",
	AtLine:             "AtLine",
	Auth:               "Auth",
	Branch:             "Branch",
	BranchFalse:        "BranchFalse",
	BranchTrue:         "BranchTrue",
	Call:               "Call",
	ClassMember:        "ClassMember",
	Coerce:             "Coerce",
	Constant:           "Constant",
	Copy:               "Copy",
	Div:                "Div",
	Drop:               "Drop",
	DropToMarker:       "DropToMarker",
	Dup:                "Dup",
	Equal:              "Equal",
	Exp:                "Exp",
	Flatten:            "Flatten",
	FromFile:           "FromFile",
	GetVarArgs:         "GetVarArgs",
	Go:                 "Go",
	GreaterThan:        "GreaterThan",
	GreaterThanOrEqual: "GreaterThanOrEqual",
	LessThan:           "LessThan",
	LessThanOrEqual:    "LessThanOrEqual",
	Load:               "Load",
	LoadIndex:          "LoadIndex",
	LoadSlice:          "LoadSlice",
	LocalCall:          "LocalCall",
	Log:                "Log",
	MakeArray:          "MakeArray",
	Member:             "Member",
	Mul:                "Mul",
	Negate:             "Negate",
	Newline:            "Newline",
	NotEqual:           "NotEqual",
	Or:                 "Or",
	Panic:              "Panic",
	PopScope:           "PopScope",
	Print:              "Print",
	Push:               "Push",
	PushScope:          "PushScope",
	RangeInit:          "RangeInit",
	RangeNext:          "RangeNext",
	RequiredType:       "RequiredType",
	Response:           "Response",
	Return:             "Return",
	Say:                "Say",
	StackCheck:         "StackCheck",
	StaticTyping:       "StaticTyping",
	Stop:               "Stop",
	Store:              "Store",
	StoreAlways:        "StoreAlways",
	StoreChan:          "StoreChan",
	StoreGlobal:        "StoreGlobal",
	StoreIndex:         "StoreIndex",
	StoreInto:          "StoreInto",
	StoreMetadata:      "StoreMetadata",
	Struct:             "Struct",
	Sub:                "Sub",
	Swap:               "Swap",
	SymbolCreate:       "SymbolCreate",
	SymbolDelete:       "SymbolDelete",
	SymbolOptCreate:    "SymbolOptCreate",
	Template:           "Template",
	This:               "This",
	Try:                "Try",
	TryPop:             "TryPop",
}

func initializeDispatch() {
	if dispatch == nil {
		dispatch = DispatchMap{
			Add:                AddImpl,
			And:                AndImpl,
			ArgCheck:           ArgCheckImpl,
			Array:              ArrayImpl,
			AtLine:             AtLineImpl,
			Auth:               AuthImpl,
			Branch:             BranchImpl,
			BranchFalse:        BranchFalseImpl,
			BranchTrue:         BranchTrueImpl,
			Call:               CallImpl,
			ClassMember:        ClassMemberImpl,
			Coerce:             CoerceImpl,
			Constant:           ConstantImpl,
			Copy:               CopyImpl,
			Div:                DivideImpl,
			Drop:               DropImpl,
			DropToMarker:       DropToMarkerImpl,
			Dup:                DupImpl,
			Equal:              EqualImpl,
			Exp:                ExponentImpl,
			Flatten:            FlattenImpl,
			FromFile:           FromFileImpl,
			GetVarArgs:         GetVarArgsImpl,
			Go:                 GoImpl,
			GreaterThan:        GreaterThanImpl,
			GreaterThanOrEqual: GreaterThanOrEqualImpl,
			LessThan:           LessThanImpl,
			LessThanOrEqual:    LessThanOrEqualImpl,
			Load:               LoadImpl,
			LoadIndex:          LoadIndexImpl,
			LoadSlice:          LoadSliceImpl,
			LocalCall:          LocalCallImpl,
			Log:                LogImpl,
			MakeArray:          MakeArrayImpl,
			Member:             MemberImpl,
			Mul:                MultiplyImpl,
			Negate:             NegateImpl,
			Newline:            NewlineImpl,
			NotEqual:           NotEqualImpl,
			Or:                 OrImpl,
			Panic:              PanicImpl,
			PopScope:           PopScopeImpl,
			Print:              PrintImpl,
			Push:               PushImpl,
			PushScope:          PushScopeImpl,
			RangeInit:          RangeInitImpl,
			RangeNext:          RangeNextImpl,
			RequiredType:       RequiredTypeImpl,
			Response:           ResponseImpl,
			Return:             ReturnImpl,
			Say:                SayImpl,
			StackCheck:         StackCheckImpl,
			StaticTyping:       StaticTypingImpl,
			Stop:               StopImpl,
			Store:              StoreImpl,
			StoreAlways:        StoreAlwaysImpl,
			StoreChan:          StoreChanImpl,
			StoreGlobal:        StoreGlobalImpl,
			StoreIndex:         StoreIndexImpl,
			StoreInto:          StoreIntoImpl,
			StoreMetadata:      StoreMetadataImpl,
			Struct:             StructImpl,
			Sub:                SubtractImpl,
			Swap:               SwapImpl,
			SymbolCreate:       SymbolCreateImpl,
			SymbolDelete:       SymbolDeleteImpl,
			SymbolOptCreate:    SymbolOptCreateImpl,
			Template:           TemplateImpl,
			This:               ThisImpl,
			Try:                TryImpl,
			TryPop:             TryPopImpl,
		}
	}
}
