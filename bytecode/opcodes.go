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
	BitAnd
	BitOr
	BitShift
	Call
	Coerce
	Constant
	Copy
	DeRef
	Div
	Drop
	DropToMarker
	Dup
	EntryPoint
	Equal
	Exp
	Explode
	Flatten
	FromFile
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
	MakeMap
	Member
	ModeCheck
	Modulo
	Mul
	Negate
	Newline
	NoOperation
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

	// After this value, additional user branch instructions
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
	BitAnd:             "BitAnd",
	BitOr:              "BitOr",
	BitShift:           "BitShift",
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
	EntryPoint:         "EntryPoint",
	Equal:              "Equal",
	Exp:                "Exp",
	Explode:            "Explode",
	Flatten:            "Flatten",
	FromFile:           "FromFile",
	GetThis:            "GetThis",
	GetVarArgs:         "GetVarArgs",
	Go:                 "Go",
	GreaterThan:        "GT",
	GreaterThanOrEqual: "GTEQ",
	LessThan:           "LT",
	LessThanOrEqual:    "LTEQ",
	Load:               "Load",
	LoadIndex:          "LoadIndex",
	LoadSlice:          "LoadSlice",
	LocalCall:          "LocalCall",
	Log:                "Log",
	MakeArray:          "MakeArray",
	MakeMap:            "MakeMap",
	Member:             "Member",
	ModeCheck:          "ModeCheck",
	Modulo:             "Modulo",
	Mul:                "Mul",
	Negate:             "Negate",
	Newline:            "Newline",
	NoOperation:        "NoOperation",
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
	StoreViaPointer:    "StorePointer",
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
			Add:                addByteCode,
			AddressOf:          addressOfByteCode,
			And:                andByteCode,
			ArgCheck:           argCheckByteCode,
			Array:              arrayByteCode,
			AtLine:             atLineByteCode,
			Auth:               authByteCode,
			BitAnd:             bitAndByteCode,
			BitOr:              bitOrByteCode,
			BitShift:           bitShiftByteCode,
			Branch:             branchByteCode,
			BranchFalse:        branchFalseByteCode,
			BranchTrue:         branchTrueByteCode,
			Call:               callByteCode,
			Coerce:             coerceByteCode,
			Constant:           constantByteCode,
			Copy:               copyByteCode,
			DeRef:              deRefByteCode,
			Div:                divideByteCode,
			Drop:               dropByteCode,
			DropToMarker:       dropToMarkerByteCode,
			Dup:                dupByteCode,
			EntryPoint:         entryPointByteCode,
			Equal:              equalByteCode,
			Exp:                exponentByteCode,
			Explode:            explodeByteCode,
			Flatten:            flattenByteCode,
			FromFile:           fromFileByteCode,
			GetThis:            getThisByteCode,
			GetVarArgs:         getVarArgsByteCode,
			Go:                 goByteCode,
			GreaterThan:        greaterThanByteCode,
			GreaterThanOrEqual: greaterThanOrEqualByteCode,
			LessThan:           lessThanByteCode,
			LessThanOrEqual:    lessThanOrEqualByteCode,
			Load:               loadByteCode,
			LoadIndex:          loadIndexByteCode,
			LoadSlice:          loadSliceByteCode,
			LocalCall:          localCallByteCode,
			Log:                logByteCode,
			MakeArray:          makeArrayByteCode,
			MakeMap:            makeMapByteCode,
			Member:             memberByteCode,
			ModeCheck:          modeCheckBytecode,
			Modulo:             moduloByteCode,
			Mul:                multiplyByteCode,
			Negate:             negateByteCode,
			Newline:            newlineByteCode,
			NoOperation:        nil,
			NotEqual:           notEqualByteCode,
			Or:                 orByteCode,
			Panic:              panicByteCode,
			PopPackage:         popPackageByteCode,
			PopScope:           popScopeByteCode,
			Print:              printByteCode,
			Push:               pushByteCode,
			PushPackage:        pushPackageByteCode,
			PushScope:          pushScopeByteCode,
			RangeInit:          rangeInitByteCode,
			RangeNext:          rangeNextByteCode,
			RequiredType:       requiredTypeByteCode,
			Response:           responseByteCode,
			Return:             returnByteCode,
			Say:                sayByteCode,
			SetThis:            setThisByteCode,
			StackCheck:         stackCheckByteCode,
			StaticTyping:       staticTypingByteCode,
			Stop:               stopByteCode,
			Store:              storeByteCode,
			StoreAlways:        storeAlwaysByteCode,
			StoreBytecode:      storeBytecodeByteCode,
			StoreChan:          storeChanByteCode,
			StoreGlobal:        storeGlobalByteCode,
			StoreIndex:         storeIndexByteCode,
			StoreInto:          storeIntoByteCode,
			StoreViaPointer:    storeViaPointerByteCode,
			Struct:             structByteCode,
			Sub:                subtractByteCode,
			Swap:               swapByteCode,
			SymbolCreate:       symbolCreateByteCode,
			SymbolDelete:       symbolDeleteByteCode,
			SymbolOptCreate:    symbolCreateIfByteCode,
			Template:           templateByteCode,
			Timer:              timerByteCode,
			Try:                tryByteCode,
			TryPop:             tryPopByteCode,
			Wait:               waitByteCode,
			WillCatch:          willCatchByteCode,
		}
	}
}
