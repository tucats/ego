package time

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

const basicLayout = "Mon Jan 2 15:04:05 MST 2006"

var timeType *data.Type
var durationType *data.Type

func Initialize(s *symbols.SymbolTable) {
	durationType = data.TypeDefinition("Duration", data.StructureType()).
		DefineField("duration", data.Int64Type).
		SetPackage("time")

	durationType.DefineFunction("String", nil, DurationString)

	structType := data.StructureType()
	structType.DefineField("time", data.InterfaceType)

	t := data.TypeDefinition("Time", structType)
	t.DefineFunction("Add", nil, Add)
	t.DefineFunction("Format", nil, Format)
	t.DefineFunction("SleepUntil", nil, SleepUntil)
	t.DefineFunction("String", nil, String)
	t.DefineFunction("Sub", nil, Sub)
	timeType = t.SetPackage("time")

	newpkg := data.NewPackageFromMap("time", map[string]interface{}{
		"Now":           Now,
		"Parse":         Parse,
		"ParseDuration": ParseDuration,
		"Since":         Since,
		"Sleep":         Sleep,
		"Time":          t,
		"Duration":      durationType,
		"Reference":     basicLayout,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
