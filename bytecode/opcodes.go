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
// function pointer of the implementation of the instruction.
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
	Arg
	Array
	BitAnd
	BitOr
	BitShift
	Call
	Coerce
	Console
	Constant
	Copy
	CreateAndStore
	Defer
	DeferStart
	DeRef
	Div
	Drop
	DropToMarker
	DumpPackages
	DumpSymbols
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
	IfError
	Import
	Increment
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
	Module
	Modulo
	Mul
	Negate
	Newline
	NoOperation
	NotEqual
	Or
	Panic
	PopScope
	Print
	Profile
	Push
	PushScope
	PushTest
	RangeInit
	ReadStack
	RequiredType
	RespHeader
	Response
	Return
	RunDefers
	Say
	Serialize
	SetThis
	Signal
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
	TryFlush
	TypeOf
	UnWrap
	Wait
	WillCatch

	// Everything from here on is a branch instruction, whose
	// operand must be present and is an integer instruction
	// address in the bytecode array. These instructions are
	// patched with offsets when code is appended.
	//
	// The first one in this list MUST be BranchInstructions,
	// as it marks the start of the branch instructions, which
	// are instructions that can reference a bytecode address
	// as the operand.
	BranchInstructions
	Branch
	BranchTrue
	BranchFalse
	LocalCall
	PopTest
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
	Arg:                "Arg",
	Array:              "Array",
	AtLine:             "AtLine",
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
	Defer:              "Defer",
	DeferStart:         "DeferStart",
	DeRef:              "DeRef",
	Div:                "Div",
	Drop:               "Drop",
	DropToMarker:       "DropToMarker",
	DumpPackages:       "DumpPackages",
	DumpSymbols:        "DumpSymbols",
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
	IfError:            "IfError",
	Import:             "Import",
	Increment:          "Increment",
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
	Module:             "Module",
	Modulo:             "Modulo",
	Mul:                "Mul",
	Negate:             "Negate",
	Newline:            "Newline",
	NoOperation:        "NoOperation",
	NotEqual:           "NotEqual",
	Or:                 "Or",
	Panic:              "Panic",
	PopScope:           "PopScope",
	PopTest:            "PopTest",
	Print:              "Print",
	Profile:            "Profile",
	Push:               "Push",
	PushScope:          "PushScope",
	PushTest:           "PushTest",
	RangeInit:          "RangeInit",
	RangeNext:          "RangeNext",
	ReadStack:          "ReadStack",
	RequiredType:       "RequiredType",
	Return:             "Return",
	RunDefers:          "RunDefers",
	Say:                "Say",
	Serialize:          "Serialize",
	SetThis:            "SetThis",
	Signal:             "Signal",
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
	TryFlush:           "TryFlush",
	TypeOf:             "TypeOf",
	UnWrap:             "UnWrap",
	Wait:               "Wait",
	WillCatch:          "WillCatch",
}

// Initialize the dispatch map. This cannot be done as a static
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
		dispatchTable[Arg] = argByteCode
		dispatchTable[Array] = arrayByteCode
		dispatchTable[AtLine] = atLineByteCode
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
		dispatchTable[Defer] = deferByteCode
		dispatchTable[DeferStart] = deferStartByteCode
		dispatchTable[DeRef] = deRefByteCode
		dispatchTable[Div] = divideByteCode
		dispatchTable[Drop] = dropByteCode
		dispatchTable[DropToMarker] = dropToMarkerByteCode
		dispatchTable[Dup] = dupByteCode
		dispatchTable[DumpPackages] = dumpPackagesByteCode
		dispatchTable[DumpSymbols] = dumpSymbolsByteCode
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
		dispatchTable[IfError] = ifErrorByteCode
		dispatchTable[Import] = importByteCode
		dispatchTable[Increment] = incrementByteCode
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
		dispatchTable[Module] = moduleByteCode
		dispatchTable[Modulo] = moduloByteCode
		dispatchTable[Mul] = multiplyByteCode
		dispatchTable[Negate] = negateByteCode
		dispatchTable[Newline] = newlineByteCode
		dispatchTable[NotEqual] = notEqualByteCode
		dispatchTable[Or] = orByteCode
		dispatchTable[Panic] = panicByteCode
		dispatchTable[PopScope] = popScopeByteCode
		dispatchTable[PopTest] = popTestByteCode
		dispatchTable[Print] = printByteCode
		dispatchTable[Profile] = profileByteCode
		dispatchTable[Push] = pushByteCode
		dispatchTable[PushScope] = pushScopeByteCode
		dispatchTable[PushTest] = pushTestByteCode
		dispatchTable[RangeInit] = rangeInitByteCode
		dispatchTable[RangeNext] = rangeNextByteCode
		dispatchTable[ReadStack] = readStackByteCode
		dispatchTable[RequiredType] = requiredTypeByteCode
		dispatchTable[Return] = returnByteCode
		dispatchTable[RunDefers] = runDefersByteCode
		dispatchTable[Say] = sayByteCode
		dispatchTable[Serialize] = serializeByteCode
		dispatchTable[SetThis] = setThisByteCode
		dispatchTable[Signal] = signalByteCode
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
		dispatchTable[TryFlush] = tryFlushByteCode
		dispatchTable[TryPop] = tryPopByteCode
		dispatchTable[TypeOf] = typeOfByteCode
		dispatchTable[UnWrap] = unwrapByteCode
		dispatchTable[Wait] = waitByteCode
		dispatchTable[WillCatch] = willCatchByteCode
	}
}
