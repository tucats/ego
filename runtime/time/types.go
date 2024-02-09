package time

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

const basicLayout = "Mon Jan 2 15:04:05 MST 2006"

var timeType *data.Type
var durationType *data.Type
var timeLock sync.Mutex

// Initialize creates the "time" package and defines it's functions and the default
// structure definition. This is serialized so it will only be done once, no matter
// how many times called.
func Initialize(s *symbols.SymbolTable) {
	timeLock.Lock()
	defer timeLock.Unlock()

	if timeType != nil {
		return
	}

	durationType = data.TypeDefinition("Duration", data.StructureType()).
		DefineField("duration", data.Int64Type).
		SetPackage("time")

	durationType.DefineFunction("String", nil, DurationString)

	structType := data.StructureType()
	structType.DefineField("time", data.InterfaceType)

	t := data.TypeDefinition("Time", structType)
	t.DefineFunction("Add", nil, add)
	t.DefineFunction("After", nil, after)
	t.DefineFunction("Before", nil, before)
	t.DefineFunction("Clock", nil, clock)
	t.DefineFunction("Format", nil, format)
	t.DefineFunction("SleepUntil", nil, sleepUntil)
	t.DefineFunction("String", nil, String)
	t.DefineFunction("Sub", nil, sub)
	timeType = t.SetPackage("time")

	newpkg := data.NewPackageFromMap("time", map[string]interface{}{
		"Now": data.Function{
			Declaration: &data.Declaration{
				Name:    "Now",
				Returns: []*data.Type{timeType},
			},
			Value: now,
		},
		"Unix": data.Function{
			Declaration: &data.Declaration{
				Name: "Unix",
				Parameters: []data.Parameter{
					{
						Name: "sec",
						Type: data.Int64Type,
					},
					{
						Name: "nsec",
						Type: data.Int64Type,
					},
				},
				Returns: []*data.Type{timeType},
			},
			Value: unix,
		},
		"Parse": data.Function{
			Declaration: &data.Declaration{
				Name: "Parse",
				Parameters: []data.Parameter{
					{
						Name: "test",
						Type: data.StringType,
					},
					{
						Name: "format",
						Type: data.StringType,
					},
				},
				ArgCount: data.Range{1, 2},
				Variadic: true,
				Returns:  []*data.Type{timeType, data.ErrorType},
			},
			Value: parseTime,
		},
		"ParseDuration": data.Function{
			Declaration: &data.Declaration{
				Name: "ParseDuration",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{durationType, data.ErrorType},
			},
			Value: parseDuration,
		},
		"Since": data.Function{
			Declaration: &data.Declaration{
				Name: "Since",
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{durationType},
			},
			Value: sinceTime,
		},
		"Sleep": data.Function{
			Declaration: &data.Declaration{
				Name: "Sleep",
				Parameters: []data.Parameter{
					{
						Name: "d",
						Type: durationType,
					},
				},
			},
			Value: sleep,
		},
		"Time":      t,
		"Duration":  durationType,
		"Reference": basicLayout,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
