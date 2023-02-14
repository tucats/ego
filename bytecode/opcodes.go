package bytecode

import (
	"sync"
)

/*
 * ADDING A NEW OPCODE
 *
 * 1. Add the Opcode name as a constant in the list below. If it is an opcode
 *    that has a bytecode address as its operand, put it in the section
 *    identified as "branch instructions".
 *
 * 2. Add the opcode name to the string map below, which converts the const identifier
 *    to a human-readable name. By convention, the human-readable name is the same as
 *    the constant itself.
 *
 * 3. Add the dispatch entry which points to the function that implements the opcode
 *    in the initializer for the dispatchTable.
 *
 * 4. Implement the actual opcode, nominally in the appropriate op_*.go file.
 */

// opcodeHandler defines a function that implements an opcode.
type opcodeHandler func(b *Context, i interface{}) error

// The dispatchTable map is a global that must be initialized once. It is an
// array indexed by the opcode (which is an integer value) and contains the
// function pointer of the implentation of the instruction.
var dispatchTable []opcodeHandler

// Mutex used to protect updating the global dispatch map.
var dispatchMux sync.Mutex

// Constant describing instruction opcodes.
type Opcode int

const (
	Stop Opcode = iota // Stop must be the zero-th item.
	AtLine
	Add
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
	Console
	Constant
	Copy
	CreateAndStore
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
	Import
	InFile
	InPackage
	LessThan
	LessThanOrEqual
	Load
	LoadIndex
	LoadSlice
	LoadThis
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
	ReadStack
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
	//
	// The first one in this list MIUST be BranchInstructions,
	// as it marks the start of the branch instructions, which
	// are instructions that can reference a bytecode address
	// as the operand.
	BranchInstructions
	Branch
	BranchTrue
	BranchFalse
	LocalCall
	RangeNext
	Try

	// This marks the end of the list.
	LastOpcode
)

var opcodeNames = map[Opcode]string{
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
	Console:            "Console",
	Constant:           "Constant",
	Copy:               "Copy",
	CreateAndStore:     "CreateAndStore",
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
	Import:             "Import",
	InFile:             "InFile",
	InPackage:          "InPackage",
	LessThan:           "LT",
	LessThanOrEqual:    "LTEQ",
	Load:               "Load",
	LoadIndex:          "LoadIndex",
	LoadSlice:          "LoadSlice",
	LoadThis:           "LoadThis",
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
	ReadStack:          "ReadStack",
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

// Iniitialize the dispatch map. This cannot be done as a static
// global initializer because some of the functions referenced
// depend on the dispatch map existing, creating an illegal
// initialization cycle.
//
// This initialization is done to a global that is accessed by
// every thread, so the map must be protected by a mutex.
func initializeDispatch() {
	dispatchMux.Lock()
	defer dispatchMux.Unlock()

	if dispatchTable == nil {
		dispatchTable = make([]opcodeHandler, LastOpcode)

		dispatchTable[Add] = addByteCode
		dispatchTable[AddressOf] = addressOfByteCode
		dispatchTable[And] = andByteCode
		dispatchTable[ArgCheck] = argCheckByteCode
		dispatchTable[Array] = arrayByteCode
		dispatchTable[AtLine] = atLineByteCode
		dispatchTable[Auth] = authByteCode
		dispatchTable[BitAnd] = bitAndByteCode
		dispatchTable[BitOr] = bitOrByteCode
		dispatchTable[BitShift] = bitShiftByteCode
		dispatchTable[Branch] = branchByteCode
		dispatchTable[BranchFalse] = branchFalseByteCode
		dispatchTable[BranchTrue] = branchTrueByteCode
		dispatchTable[Call] = callByteCode
		dispatchTable[Coerce] = coerceByteCode
		dispatchTable[Console] = consoleByteCode
		dispatchTable[Constant] = constantByteCode
		dispatchTable[Copy] = copyByteCode
		dispatchTable[CreateAndStore] = createAndStoreByteCode
		dispatchTable[DeRef] = deRefByteCode
		dispatchTable[Div] = divideByteCode
		dispatchTable[Drop] = dropByteCode
		dispatchTable[DropToMarker] = dropToMarkerByteCode
		dispatchTable[Dup] = dupByteCode
		dispatchTable[EntryPoint] = entryPointByteCode
		dispatchTable[Equal] = equalByteCode
		dispatchTable[Exp] = exponentByteCode
		dispatchTable[Explode] = explodeByteCode
		dispatchTable[Flatten] = flattenByteCode
		dispatchTable[FromFile] = fromFileByteCode
		dispatchTable[GreaterThan] = greaterThanByteCode
		dispatchTable[GreaterThanOrEqual] = greaterThanOrEqualByteCode
		dispatchTable[GetThis] = getThisByteCode
		dispatchTable[GetVarArgs] = getVarArgsByteCode
		dispatchTable[Go] = goByteCode
		dispatchTable[Import] = importByteCode
		dispatchTable[InFile] = inFileByteCode
		dispatchTable[InPackage] = inPackageByteCode
		dispatchTable[LessThan] = lessThanByteCode
		dispatchTable[LessThanOrEqual] = lessThanOrEqualByteCode
		dispatchTable[Load] = loadByteCode
		dispatchTable[LoadIndex] = loadIndexByteCode
		dispatchTable[LoadSlice] = loadSliceByteCode
		dispatchTable[LoadThis] = loadThisByteCode
		dispatchTable[LocalCall] = localCallByteCode
		dispatchTable[Log] = logByteCode
		dispatchTable[MakeArray] = makeArrayByteCode
		dispatchTable[MakeMap] = makeMapByteCode
		dispatchTable[Member] = memberByteCode
		dispatchTable[ModeCheck] = modeCheckBytecode
		dispatchTable[Modulo] = moduloByteCode
		dispatchTable[Mul] = multiplyByteCode
		dispatchTable[Negate] = negateByteCode
		dispatchTable[Newline] = newlineByteCode
		dispatchTable[NotEqual] = notEqualByteCode
		dispatchTable[Or] = orByteCode
		dispatchTable[Panic] = panicByteCode
		dispatchTable[PopPackage] = popPackageByteCode
		dispatchTable[PopScope] = popScopeByteCode
		dispatchTable[Print] = printByteCode
		dispatchTable[Push] = pushByteCode
		dispatchTable[PushPackage] = pushPackageByteCode
		dispatchTable[PushScope] = pushScopeByteCode
		dispatchTable[RangeInit] = rangeInitByteCode
		dispatchTable[RangeNext] = rangeNextByteCode
		dispatchTable[ReadStack] = readStackByteCode
		dispatchTable[RequiredType] = requiredTypeByteCode
		dispatchTable[Response] = responseByteCode
		dispatchTable[Return] = returnByteCode
		dispatchTable[Say] = sayByteCode
		dispatchTable[SetThis] = setThisByteCode
		dispatchTable[StackCheck] = stackCheckByteCode
		dispatchTable[StaticTyping] = staticTypingByteCode
		dispatchTable[Stop] = stopByteCode
		dispatchTable[Store] = storeByteCode
		dispatchTable[StoreAlways] = storeAlwaysByteCode
		dispatchTable[StoreBytecode] = storeBytecodeByteCode
		dispatchTable[StoreChan] = storeChanByteCode
		dispatchTable[StoreGlobal] = storeGlobalByteCode
		dispatchTable[StoreIndex] = storeIndexByteCode
		dispatchTable[StoreInto] = storeIntoByteCode
		dispatchTable[StoreViaPointer] = storeViaPointerByteCode
		dispatchTable[Struct] = structByteCode
		dispatchTable[Sub] = subtractByteCode
		dispatchTable[Swap] = swapByteCode
		dispatchTable[SymbolCreate] = symbolCreateByteCode
		dispatchTable[SymbolDelete] = symbolDeleteByteCode
		dispatchTable[SymbolOptCreate] = symbolCreateIfByteCode
		dispatchTable[Template] = templateByteCode
		dispatchTable[Timer] = timerByteCode
		dispatchTable[Try] = tryByteCode
		dispatchTable[TryPop] = tryPopByteCode
		dispatchTable[Wait] = waitByteCode
		dispatchTable[WillCatch] = willCatchByteCode

	}
}
