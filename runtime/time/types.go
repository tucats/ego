package time

import (
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
)

var TimeType = data.TypeDefinition("Time", data.StructType).
	SetNativeName("time.Time").
	SetPackage("time").
	DefineNativeFunction("Add",
		&data.Declaration{
			Name: "Add",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "d",
					Type: TimeDurationType,
				},
			},
			Returns: []*data.Type{data.OwnType},
		}, nil).
	DefineNativeFunction("After", &data.Declaration{
		Name: "After",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "t",
				Type: data.OwnType,
			},
		},
		Returns: []*data.Type{data.BoolType},
	}, nil).
	DefineNativeFunction("Before", &data.Declaration{
		Name: "Before",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "t",
				Type: data.OwnType,
			},
		},
		Returns: []*data.Type{data.BoolType},
	}, nil).
	DefineNativeFunction("Clock", &data.Declaration{
		Name:    "Clock",
		Type:    data.OwnType,
		Returns: []*data.Type{data.IntType, data.IntType, data.IntType},
	}, nil).
	DefineNativeFunction("Equal", &data.Declaration{
		Name: "Equal",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "t1",
				Type: data.OwnType,
			},
		},
		Returns: []*data.Type{data.BoolType},
	}, nil).
	DefineNativeFunction("Format", &data.Declaration{
		Name: "Format",
		Type: data.OwnType,
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
		Type:    data.OwnType,
		Returns: []*data.Type{data.IntType},
	}, nil).
	DefineNativeFunction("Month", &data.Declaration{
		Name:    "Month",
		Type:    data.OwnType,
		Returns: []*data.Type{TimeMonthType},
	}, nil).
	DefineNativeFunction("String", &data.Declaration{
		Name:    "String",
		Type:    data.OwnType,
		Returns: []*data.Type{data.StringType},
	}, nil).
	DefineNativeFunction("Sub", &data.Declaration{
		Name: "Sub",
		Type: data.OwnType,
		Parameters: []data.Parameter{
			{
				Name: "t",
				Type: data.OwnType,
			},
		},
		Returns: []*data.Type{TimeDurationType},
	}, nil)

var TimeDurationType = data.TypeDefinition("Duration", data.StructureType()).
	SetNativeName(defs.TimeDurationTypeName).
	SetPackage("time").
	DefineFunction("String",
		&data.Declaration{
			Name: "String",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "extendedFormat",
					Type: data.BoolType,
				},
			},
			ArgCount: data.Range{0, 1},
			Returns:  []*data.Type{data.StringType},
		}, durationString).
	DefineNativeFunction("Hours",
		&data.Declaration{
			Name:    "Hours",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Float64Type},
		}, nil).
	DefineNativeFunction("Minutes",
		&data.Declaration{
			Name:    "Minutes",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Float64Type},
		}, nil).
	DefineNativeFunction("Seconds",
		&data.Declaration{
			Name:    "Seconds",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Float64Type},
		}, nil).
	DefineNativeFunction("Milliseconds",
		&data.Declaration{
			Name:    "Milliseconds",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Float64Type},
		}, nil).
	DefineNativeFunction("Microseconds",
		&data.Declaration{
			Name:    "Microseconds",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Float64Type},
		}, nil).
	DefineNativeFunction("Nanoseconds",
		&data.Declaration{
			Name:    "Nanoseconds",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Float64Type},
		}, nil)

var TimeLocationType = data.TypeDefinition("Location", data.PointerType(data.StructureType())).
	SetNativeName(defs.TimeLocationTypeName).
	SetPackage("time").
	DefineNativeFunction("String",
		&data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, nil)

var TimeMonthType = data.TypeDefinition("Month", data.StructureType()).
	SetNativeName(defs.TimeMonthTypeName).
	SetPackage("time").
	DefineNativeFunction("String",
		&data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, nil)

var TimePackage = data.NewPackageFromMap("time", map[string]interface{}{
	"Now": data.Function{
		Declaration: &data.Declaration{
			Name:    "Now",
			Returns: []*data.Type{TimeType},
		},
		Value:    time.Now,
		IsNative: true,
	},
	"Date": data.Function{
		Declaration: &data.Declaration{
			Name:    "Date",
			Returns: []*data.Type{TimeType},
			Parameters: []data.Parameter{
				{
					Name: "year",
					Type: data.IntType,
				},
				{
					Name: "month",
					Type: TimeMonthType,
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
					Type: data.PointerType(TimeLocationType),
				},
			},
		},
		Value:    time.Date,
		IsNative: true,
	},
	"FixedZone": data.Function{
		Declaration: &data.Declaration{
			Name:    "FixedZone",
			Returns: []*data.Type{data.PointerType(TimeLocationType)},
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
			Returns: []*data.Type{data.PointerType(TimeLocationType), data.ErrorType},
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
			Returns: []*data.Type{TimeType},
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
			Returns: []*data.Type{TimeType, data.ErrorType},
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
			Returns: []*data.Type{TimeDurationType, data.ErrorType},
		},
		Value: parseDuration,
	},
	"Since": data.Function{
		Declaration: &data.Declaration{
			Name: "Since",
			Parameters: []data.Parameter{
				{
					Name: "t",
					Type: TimeType,
				},
			},
			Returns: []*data.Type{TimeDurationType},
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
					Type: TimeDurationType,
				},
			},
		},
		Value:    time.Sleep,
		IsNative: true,
	},
	"Time":     TimeType,
	"Duration": TimeDurationType,
	"Location": TimeLocationType,
	"Month":    TimeMonthType,
})
