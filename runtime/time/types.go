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
var initLock sync.Mutex

// Initialize creates the "time" package and defines it's functions and the default
// structure definition. This is serialized so it will only be done once, no matter
// how many times called.
func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if timeType == nil {
		durationType = data.TypeDefinition("Duration", data.StructureType()).
			SetNativeName("time.Duration").
			SetPackage("time")

		durationType.DefineFunction("String",
			&data.Declaration{
				Name: "String",
				Parameters: []data.Parameter{
					{
						Name: "extendedFormat",
						Type: data.BoolType,
					},
				},
				ArgCount: data.Range{0, 1},
				Returns:  []*data.Type{data.StringType},
			}, durationString)

		durationType.DefineFunction("Hours",
			&data.Declaration{
				Name:    "Hours",
				Returns: []*data.Type{data.Float64Type},
			}, durationHours)

		durationType.DefineFunction("Minutes",
			&data.Declaration{
				Name:    "Minutes",
				Returns: []*data.Type{data.Float64Type},
			}, durationMinutes)

		durationType.DefineFunction("Seconds",
			&data.Declaration{
				Name:    "Seconds",
				Returns: []*data.Type{data.Float64Type},
			}, durationSeconds)

		durationType.DefineFunction("Milliseconds",
			&data.Declaration{
				Name:    "Milliseconds",
				Returns: []*data.Type{data.Float64Type},
			}, durationMilliseconds)

		durationType.DefineFunction("Microseconds",
			&data.Declaration{
				Name:    "Microseconds",
				Returns: []*data.Type{data.Float64Type},
			}, durationMicroseconds)

		durationType.DefineFunction("Nanoseconds",
			&data.Declaration{
				Name:    "Nanoseconds",
				Returns: []*data.Type{data.Float64Type},
			}, durationNanoseconds)

		structType := data.StructureType()

		// Create the Time type as a native instance of a *time.Time and add in the
		// built-in functions. To prevent chicken-egg issue, define timeType as a type
		// before filling it in, so functions can reference the type in their function
		// declarations
		timeType = data.TypeDefinition("Time", structType)
		timeType.SetNativeName("*time.Time").
			SetPackage("time").
			DefineFunction("Add",
				&data.Declaration{
					Name: "Add",
					Parameters: []data.Parameter{
						{
							Name: "d",
							Type: durationType,
						},
					},
					Returns: []*data.Type{timeType},
				}, add).
			DefineFunction("After", &data.Declaration{
				Name: "After",
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{data.BoolType},
			}, after).
			DefineFunction("Before", &data.Declaration{
				Name: "Before",
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{data.BoolType},
			}, before).
			DefineFunction("Clock", &data.Declaration{
				Name:    "Clock",
				Returns: []*data.Type{data.IntType, data.IntType, data.IntType},
			}, clock).
			DefineFunction("Format", &data.Declaration{
				Name: "Format",
				Parameters: []data.Parameter{
					{
						Name: "layout",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			}, format).
			DefineFunction("SleepUntil", &data.Declaration{
				Name: "SleepUntil",
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
			}, sleepUntil).
			DefineFunction("String", &data.Declaration{
				Name:    "String",
				Returns: []*data.Type{data.StringType},
			}, String).
			DefineFunction("Sub", &data.Declaration{
				Name: "Sub",
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{durationType},
			}, sub)
	}

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
		"Time":      timeType,
		"Duration":  durationType,
		"Reference": basicLayout,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}

// GetTimeType returns the time.Time type.
func GetTimeType(s *symbols.SymbolTable) *data.Type {
	if timeType == nil {
		Initialize(s)
	}

	return timeType
}
