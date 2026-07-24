package time

import (
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/data"
)

var TimeWeekdayType = data.TypeDefinition("Weekday", data.IntType).
	SetPackage("time").
	DefineFunction("String",
		&data.Declaration{
			Name:    "String",
			Returns: []*data.Type{data.StringType},
		}, weekdayString)

var TimeType = data.TypeDefinition("Time", data.StructureType()).
	SetNativeName("time.Time").
	SetPackage("time").
	// Fix BUG-56: without a format function, data.Format()'s default case
	// falls back to reflection-based formatting of the native Go struct
	// layout (e.g. "time.Time struct{wall: uint64, ...} = ...") instead of
	// the human-readable string that .String() and %v already produce.
	SetFormatFunc(func(v any) string {
		t, _ := v.(time.Time)

		return t.String()
	}).
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
	}, nil).
	// SleepUntil is an Ego-specific addition with no Go equivalent -- Go
	// code does the same thing with time.Sleep(time.Until(t)).
	DefineFunction("SleepUntil", &data.Declaration{
		Name:    "SleepUntil",
		Type:    data.OwnType,
		Returns: []*data.Type{data.ErrorType},
	}, sleepUntil).
	// SleepUntil is an Ego-specific addition with no Go equivalent -- Go
	// code does the same thing with time.Sleep(time.Until(t)).
	DefineFunction("Weekday", &data.Declaration{
		Name:    "Weekday",
		Type:    data.OwnType,
		Returns: []*data.Type{TimeWeekdayType},
	}, weekday).
	FixSelfReferences()

var TimeDurationType = data.TypeDefinition("Duration", data.StructureType()).
	SetNativeName(defs.TimeDurationTypeName).
	SetPackage("time").
	// Fix BUG-56: same reasoning as TimeType above -- without this, printing
	// a bare time.Duration shows "time.Duration int64 1h30m0s" instead of
	// "1h30m0s".
	SetFormatFunc(func(v any) string {
		d, _ := v.(time.Duration)

		return d.String()
	}).
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
			Returns:  []*data.Type{data.StringType, data.ErrorType},
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
			Returns: []*data.Type{data.Int64Type},
		}, nil).
	DefineNativeFunction("Microseconds",
		&data.Declaration{
			Name:    "Microseconds",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Int64Type},
		}, nil).
	DefineNativeFunction("Nanoseconds",
		&data.Declaration{
			Name:    "Nanoseconds",
			Type:    data.OwnType,
			Returns: []*data.Type{data.Int64Type},
		}, nil).FixSelfReferences()

var TimeLocationType = data.TypeDefinition("Location", data.PointerType(data.StructureType())).
	SetNativeName(defs.TimeLocationTypeName).
	SetPackage("time").
	// Same BUG-56 root cause as TimeType/TimeDurationType above, found during
	// the same survey: *time.Location is handed around as a pointer, so
	// without this the default formatter dumps its entire internal zone
	// table instead of calling its own String() method.
	SetFormatFunc(func(v any) string {
		l, _ := v.(*time.Location)
		if l == nil {
			return ""
		}

		return l.String()
	}).
	DefineNativeFunction("String",
		&data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, nil).FixSelfReferences()

var TimeMonthType = data.TypeDefinition("Month", data.StructureType()).
	SetNativeName(defs.TimeMonthTypeName).
	SetPackage("time").
	// Same BUG-56 root cause as above, found during the same survey:
	// without this, printing a bare time.Month shows "time.Month int July"
	// instead of "July".
	SetFormatFunc(func(v any) string {
		m, _ := v.(time.Month)

		return m.String()
	}).
	DefineNativeFunction("String",
		&data.Declaration{
			Name:    "String",
			Type:    data.OwnType,
			Returns: []*data.Type{data.StringType},
		}, nil).FixSelfReferences()

var TimePackage = data.NewPackageFromMap("time", map[string]any{
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
					Name: "nanoseconds",
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
	"ParseAny": data.Function{
		Declaration: &data.Declaration{
			Name: "ParseAny",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{TimeType, data.ErrorType},
		},
		Value: Parse,
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
	"Weekday":  TimeWeekdayType,
})
