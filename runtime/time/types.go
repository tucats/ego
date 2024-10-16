package time

import (
	"sync"
	"time"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var timeType *data.Type
var durationType *data.Type
var locationType *data.Type
var monthType *data.Type
var initLock sync.Mutex

// Initialize creates the "time" package and defines it's functions and the default
// structure definition. This is serialized so it will only be done once, no matter
// how many times called.
func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if timeType == nil {
		monthType = data.TypeDefinition("Month", data.StructureType()).
			SetNativeName("time.Month").
			SetPackage("time")

		monthType.DefineNativeFunction("String",
			&data.Declaration{
				Name:    "String",
				Type:    monthType,
				Returns: []*data.Type{data.StringType},
			}, nil)

		locationType = data.TypeDefinition("Location", data.PointerType(data.StructureType())).
			SetNativeName("time.Location").
			SetPackage("time")

		locationType.DefineNativeFunction("String",
			&data.Declaration{
				Name:    "String",
				Type:    locationType,
				Returns: []*data.Type{data.StringType},
			}, nil)

		timeType = data.TypeDefinition("Time", data.StructureType())
		durationType = data.TypeDefinition("Duration", data.StructureType()).
			SetNativeName("time.Duration").
			SetPackage("time")

		durationType.DefineFunction("String",
			&data.Declaration{
				Name: "String",
				Type: durationType,
				Parameters: []data.Parameter{
					{
						Name: "extendedFormat",
						Type: data.BoolType,
					},
				},
				ArgCount: data.Range{0, 1},
				Returns:  []*data.Type{data.StringType},
			}, durationString)

		durationType.DefineNativeFunction("Hours",
			&data.Declaration{
				Name:    "Hours",
				Type:    durationType,
				Returns: []*data.Type{data.Float64Type},
			}, nil)

		durationType.DefineNativeFunction("Minutes",
			&data.Declaration{
				Name:    "Minutes",
				Type:    durationType,
				Returns: []*data.Type{data.Float64Type},
			}, nil)

		durationType.DefineNativeFunction("Seconds",
			&data.Declaration{
				Name:    "Seconds",
				Type:    durationType,
				Returns: []*data.Type{data.Float64Type},
			}, nil)

		durationType.DefineNativeFunction("Milliseconds",
			&data.Declaration{
				Name:    "Milliseconds",
				Type:    durationType,
				Returns: []*data.Type{data.Float64Type},
			}, nil)

		durationType.DefineNativeFunction("Microseconds",
			&data.Declaration{
				Name:    "Microseconds",
				Type:    durationType,
				Returns: []*data.Type{data.Float64Type},
			}, nil)

		durationType.DefineNativeFunction("Nanoseconds",
			&data.Declaration{
				Name:    "Nanoseconds",
				Type:    durationType,
				Returns: []*data.Type{data.Float64Type},
			}, nil)

		structType := data.StructureType()

		// Create the Time type as a native instance of a *time.Time and add in the
		// built-in functions. To prevent chicken-egg issue, define timeType as a type
		// before filling it in, so functions can reference the type in their function
		// declarations
		timeType = data.TypeDefinition("Time", structType)
		timeType.SetNativeName("time.Time").
			SetPackage("time").
			DefineNativeFunction("Add",
				&data.Declaration{
					Name: "Add",
					Type: timeType,
					Parameters: []data.Parameter{
						{
							Name: "d",
							Type: durationType,
						},
					},
					Returns: []*data.Type{timeType},
				}, nil).
			DefineNativeFunction("After", &data.Declaration{
				Name: "After",
				Type: timeType,
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{data.BoolType},
			}, nil).
			DefineNativeFunction("Before", &data.Declaration{
				Name: "Before",
				Type: timeType,
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{data.BoolType},
			}, nil).
			DefineNativeFunction("Clock", &data.Declaration{
				Name:    "Clock",
				Type:    timeType,
				Returns: []*data.Type{data.IntType, data.IntType, data.IntType},
			}, nil).
			DefineNativeFunction("Format", &data.Declaration{
				Name: "Format",
				Type: timeType,
				Parameters: []data.Parameter{
					{
						Name: "layout",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType},
			}, nil).
			DefineNativeFunction("Hour", &data.Declaration{
				Name:    "Hour",
				Type:    timeType,
				Returns: []*data.Type{data.IntType},
			}, nil).
			DefineNativeFunction("Month", &data.Declaration{
				Name:    "Month",
				Type:    timeType,
				Returns: []*data.Type{monthType},
			}, nil).
			DefineNativeFunction("String", &data.Declaration{
				Name:    "String",
				Type:    timeType,
				Returns: []*data.Type{data.StringType},
			}, nil).
			DefineNativeFunction("Sub", &data.Declaration{
				Name: "Sub",
				Type: timeType,
				Parameters: []data.Parameter{
					{
						Name: "t",
						Type: timeType,
					},
				},
				Returns: []*data.Type{durationType},
			}, nil)
	}

	if _, found := s.Root().Get("time"); !found {
		newpkg := data.NewPackageFromMap("time", map[string]interface{}{
			"Now": data.Function{
				Declaration: &data.Declaration{
					Name:    "Now",
					Returns: []*data.Type{timeType},
				},
				Value:    time.Now,
				IsNative: true,
			},
			"Date": data.Function{
				Declaration: &data.Declaration{
					Name:    "Date",
					Returns: []*data.Type{timeType},
					Parameters: []data.Parameter{
						{
							Name: "year",
							Type: data.IntType,
						},
						{
							Name: "month",
							Type: monthType,
						},
						{
							Name: "day",
							Type: data.IntType,
						},
						{
							Name: "hour",
							Type: data.IntType,
						},
						{
							Name: "minute",
							Type: data.IntType,
						},
						{
							Name: "second",
							Type: data.IntType,
						},
						{
							Name: "nanosecond",
							Type: data.IntType,
						},
						{
							Name: "location",
							Type: data.PointerType(locationType),
						},
					},
				},
				Value:    time.Date,
				IsNative: true,
			},
			"FixedZone": data.Function{
				Declaration: &data.Declaration{
					Name:    "FixedZone",
					Returns: []*data.Type{data.PointerType(locationType)},
					Parameters: []data.Parameter{
						{
							Name: "name",
							Type: data.StringType,
						},
						{
							Name: "offset",
							Type: data.IntType,
						},
					},
				},
				Value:    time.FixedZone,
				IsNative: true,
			},
			"LoadLocation": data.Function{
				Declaration: &data.Declaration{
					Name:    "LoadLocation",
					Returns: []*data.Type{data.PointerType(locationType), data.ErrorType},
					Parameters: []data.Parameter{
						{
							Name: "name",
							Type: data.StringType,
						},
					},
				},
				Value:    time.LoadLocation,
				IsNative: true,
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
				Value:    time.Unix,
				IsNative: true,
			},
			"Parse": data.Function{
				Declaration: &data.Declaration{
					Name: "Parse",
					Parameters: []data.Parameter{
						{
							Name: "format",
							Type: data.StringType,
						},
						{
							Name: "text",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{timeType, data.ErrorType},
				},
				Value:    time.Parse,
				IsNative: true,
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
				Value:    time.ParseDuration,
				IsNative: true,
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
				Value:    time.Since,
				IsNative: true,
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
				Value:    time.Sleep,
				IsNative: true,
			},
			"Time":     timeType,
			"Duration": durationType,
			"Location": locationType,
			"Month":    monthType,
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}

// GetTimeType returns the time.Time type.
func GetTimeType(s *symbols.SymbolTable) *data.Type {
	if timeType == nil {
		Initialize(s)
	}

	return timeType
}
