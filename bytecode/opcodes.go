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

// Constant describing instruction opcodes.
type OpcodeID int

const (
	Stop   OpcodeID = 0
	AtLine          = iota + BuiltinInstructions
	Add    OpcodeID = iota
	AddressOf
	And
	ArgCheck
	Array
	Auth
	Call
	Coerce
	Constant
	Copy
	DeRef
	Div
	Drop
	DropToMarker
	Dup
	Equal
	Exp
	Explode
	Flatten
	FromFile
	GetRegister
	GetThis
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
	ModeCheck
	Mul
	Negate
	Newline
	NotEqual
	Or
	Panic
	PopPackage
	PopScope
	Print
	Push
	PushPackage
	PushScope
	RangeInit
	RangeNext
	RequiredType
	Response
	Return
	Say
	SetRegister
	SetThis
	StackCheck
	StaticTyping
	Store
	StoreAlways
	StoreBytecode
	StoreChan
	StoreGlobal
	StoreIndex
	StoreInto
	StoreMetadata
	StoreViaPointer
	Struct
	Sub
	Swap
	SymbolCreate
	SymbolDelete
	SymbolOptCreate
	Template
	Timer
	TryPop
	Wait
	WillCatch

	// Everything from here on is a branch instruction, whose
	// operand must be present and is an integer instruction
	// address in the bytecode array. These instructions are
	// patched with offsets when code is appended.
	BranchInstructions = iota + BranchInstruction
	Branch
	BranchTrue
	BranchFalse
	LocalCall
	Try

	// After this value, additional user branch instructions are
	// can be defined.
	UserBranchInstructions
)

var instructionNames = map[OpcodeID]string{
	Add:                "Add",
	AddressOf:          "AddressOf",
	And:                "And",
	ArgCheck:           "ArgCheck",
	Array:              "Array",
	AtLine:             "AtLine",
	Auth:               "Auth",
	Branch:             "Branch",
	BranchFalse:        "BranchFalse",
	BranchTrue:         "BranchTrue",
	Call:               "Call",
	Coerce:             "Coerce",
	Constant:           "Constant",
	Copy:               "Copy",
	DeRef:              "DeRef",
	Div:                "Div",
	Drop:               "Drop",
	DropToMarker:       "DropToMarker",
	Dup:                "Dup",
	Equal:              "Equal",
	Exp:                "Exp",
	Explode:            "Explode",
	Flatten:            "Flatten",
	FromFile:           "FromFile",
	GetRegister:        "GetRegister",
	GetThis:            "GetThis",
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
	ModeCheck:          "ModeCheck",
	Negate:             "Negate",
	Newline:            "Newline",
	NotEqual:           "NotEqual",
	Or:                 "Or",
	Panic:              "Panic",
	PopPackage:         "PopPackage",
	PopScope:           "PopScope",
	Print:              "Print",
	Push:               "Push",
	PushPackage:        "PushPackage",
	PushScope:          "PushScope",
	RangeInit:          "RangeInit",
	RangeNext:          "RangeNext",
	RequiredType:       "RequiredType",
	Response:           "Response",
	Return:             "Return",
	Say:                "Say",
	SetRegister:        "SetRegister",
	SetThis:            "SetThis",
	StackCheck:         "StackCheck",
	StaticTyping:       "StaticTyping",
	Stop:               "Stop",
	Store:              "Store",
	StoreAlways:        "StoreAlways",
	StoreBytecode:      "StoreBytecode",
	StoreChan:          "StoreChan",
	StoreGlobal:        "StoreGlobal",
	StoreIndex:         "StoreIndex",
	StoreInto:          "StoreInto",
	StoreMetadata:      "StoreMetadata",
	StoreViaPointer:    "StoreViaPointer",
	Struct:             "Struct",
	Sub:                "Sub",
	Swap:               "Swap",
	SymbolCreate:       "SymbolCreate",
	SymbolDelete:       "SymbolDelete",
	SymbolOptCreate:    "SymbolOptCreate",
	Template:           "Template",
	Timer:              "Timer",
	Try:                "Try",
	TryPop:             "TryPop",
	Wait:               "Wait",
	WillCatch:          "WillCatch",
}

func initializeDispatch() {
	if dispatch == nil {
		dispatch = DispatchMap{
			Add:                AddImpl,
			AddressOf:          AddressOfImpl,
			And:                AndImpl,
			ArgCheck:           ArgCheckImpl,
			Array:              ArrayImpl,
			AtLine:             AtLineImpl,
			Auth:               AuthImpl,
			Branch:             BranchImpl,
			BranchFalse:        BranchFalseImpl,
			BranchTrue:         BranchTrueImpl,
			Call:               CallImpl,
			Coerce:             CoerceImpl,
			Constant:           constantByteCode,
			Copy:               CopyImpl,
			DeRef:              DeRefImpl,
			Div:                DivideImpl,
			Drop:               DropImpl,
			DropToMarker:       DropToMarkerImpl,
			Dup:                DupImpl,
			Equal:              EqualImpl,
			Exp:                ExponentImpl,
			Explode:            ExplodeImpl,
			Flatten:            FlattenImpl,
			FromFile:           FromFileImpl,
			GetRegister:        getRegister,
			GetThis:            GetThisImpl,
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
			Member:             memberByteCode,
			ModeCheck:          modeCheckBytecode,
			Mul:                MultiplyImpl,
			Negate:             NegateImpl,
			Newline:            NewlineImpl,
			NotEqual:           NotEqualImpl,
			Or:                 OrImpl,
			Panic:              PanicImpl,
			PopPackage:         popPackage,
			PopScope:           PopScopeImpl,
			Print:              PrintImpl,
			Push:               PushImpl,
			PushPackage:        pushPackage,
			PushScope:          PushScopeImpl,
			RangeInit:          RangeInitImpl,
			RangeNext:          RangeNextImpl,
			RequiredType:       RequiredTypeImpl,
			Response:           ResponseImpl,
			Return:             ReturnImpl,
			Say:                SayImpl,
			SetRegister:        setRegister,
			SetThis:            SetThisImpl,
			StackCheck:         StackCheckImpl,
			StaticTyping:       StaticTypingImpl,
			Stop:               StopImpl,
			Store:              StoreImpl,
			StoreAlways:        StoreAlwaysImpl,
			StoreBytecode:      StoreBytecodeImpl,
			StoreChan:          StoreChanImpl,
			StoreGlobal:        StoreGlobalImpl,
			StoreIndex:         storeIndexByteCode,
			StoreInto:          StoreIntoImpl,
			StoreMetadata:      StoreMetadataImpl,
			StoreViaPointer:    StoreViaPointerImpl,
			Struct:             structByteCode,
			Sub:                SubtractImpl,
			Swap:               SwapImpl,
			SymbolCreate:       SymbolCreateImpl,
			SymbolDelete:       SymbolDeleteImpl,
			SymbolOptCreate:    SymbolOptCreateImpl,
			Template:           TemplateImpl,
			Timer:              TimerImpl,
			Try:                TryImpl,
			TryPop:             TryPopImpl,
			Wait:               wait,
			WillCatch:          willCatch,
		}
	}
}
