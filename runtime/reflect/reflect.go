package reflect

import (
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// describe implements the reflect.Reflect() function.
func describe(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	source := args.Get(0)
	vv := reflect.ValueOf(source)
	ts := vv.String()

	// See if the text representation of the value matches a builtin function. If
	// so, get the full function path from the PC value of the function, and use
	// that to extract the function declaration.
	if ts == defs.RuntimeFunctionReflectionTypeString {
		return describeBuiltinFunction(source)
	}

	// If it's a bytecode.Bytecode pointer, use native reflection to get the
	// Name field value from the bytecode object to get the function name.
	// Note that anonymous functions will have a name of "".
	if vv.Kind() == reflect.Ptr {
		if ts == defs.ByteCodeReflectionTypeString {
			return describeBytecodeFunction(source)
		}
	}

	// If it's a runtime function, use the predefined declaration info for
	// the runtime to return the function info. If there is no declaration,
	// this is a legacy function definition, so log it an then return the
	// function name as a string (These should all be cleaned up)
	if m, ok := source.(data.Function); ok {
		if m.Declaration == nil {
			text := data.Format(m.Value)

			return text, nil
		}

		return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
			data.TypeMDName:        funcLabel,
			data.BasetypeMDName:    funcLabel + " " + m.Declaration.Name,
			data.IsTypeMDName:      false,
			data.DeclarationMDName: makeDeclaration(m.Declaration),
			data.NameMDName:        m.Declaration.Name,
			data.NativeMDName:      true,
		}), nil
	}

	// If it's a structure, in addition to the structure metadata, we will
	// search for type methods that work against this structure type.
	if s, ok := source.(*data.Struct); ok {
		m := map[string]interface{}{}

		m[data.TypeMDName] = s.TypeString()
		if s.Type().IsTypeDefinition() {
			m[data.BasetypeMDName] = s.Type().BaseType().String()
		} else {
			m[data.BasetypeMDName] = s.Type().String()
		}

		// If there are methods associated with this type, add them to the output structure.
		methods := s.Type().FunctionNames()
		if len(methods) > 0 {
			names := make([]interface{}, 0)

			for _, name := range methods {
				if name > "" {
					names = append(names, name)
				}
			}

			m[data.FunctionsMDName] = data.NewArrayFromInterfaces(data.StringType, names...)
		}

		m[data.IsTypeMDName] = false
		m[data.NativeMDName] = false
		m[data.MembersMDName] = s.FieldNamesArray(true)
		m[data.PackageMDName] = s.PackageName()

		return data.NewStructOfTypeFromMap(ReflectReflectionType, m), nil
	}

	// Similarly, if it's a type, then we check to see if it's a user type
	// definition. If so, we return the type metadata for associated
	// functions, etc.
	if t, ok := source.(*data.Type); ok {
		r := map[string]interface{}{}

		r[data.NativeMDName] = t.IsBaseType()
		r[data.IsTypeMDName] = true
		r[data.BasetypeMDName] = t.TypeString()
		r[data.TypeMDName] = t.TypeString()

		if t.IsTypeDefinition() {
			r[data.BasetypeMDName] = t.BaseType().TypeString()
			r[data.TypeMDName] = "type"
		}

		if t.Name() != "" {
			r[data.NameMDName] = t.Name()
		}

		functionList := t.FunctionNames()
		if t.BaseType() != nil && t.BaseType().Kind() == data.InterfaceKind {
			functionList = append(functionList, t.BaseType().FunctionNames()...)
		}

		if len(functionList) > 0 {
			functions := data.NewArray(data.StringType, len(functionList))

			sort.Strings(functionList)

			for i, k := range functionList {
				var fd data.Function

				if v := t.Function(k); v != nil {
					if f, ok := v.(data.Function); ok {
						fd = f
					}
				}

				fName := fd
				name := k

				if fName.Declaration != nil {
					name = fName.Declaration.String()
				}

				_ = functions.Set(i, name)
			}

			r[data.FunctionsMDName] = functions
		}

		return data.NewStructOfTypeFromMap(ReflectReflectionType, r), nil
	}

	// Is it an Ego package? Return information about whether the package
	// includes native builtin functions and/or Ego functions imported as
	// source from the library.
	if packageDef, ok := source.(*data.Package); ok {
		// Make a list of the visible member names
		memberList := []string{}

		for _, k := range packageDef.Keys() {
			if !strings.HasPrefix(k, data.MetadataPrefix) {
				memberList = append(memberList, k)
			}
		}

		// Also need to grab any exported symbols in the package's symbol
		// table not alrady included in the package items list.

		// Also need to collect any exported symbols from the package.
		symbolTable := symbols.GetPackageSymbolTable(packageDef)
		for _, k := range symbolTable.Names() {
			// If invisible, ignore
			if strings.HasPrefix(k, defs.InvisiblePrefix) {
				continue
			}

			// If not exporited, ignore
			if !egostrings.HasCapitalizedName(k) {
				continue
			}

			// If already in the array, ignore
			if _, found := packageDef.Get(k); found {
				continue
			}

			memberList = append(memberList, k)
		}

		// Sort the member list and forge it into an Ego array
		members := util.MakeSortedArray(memberList)

		result := map[string]interface{}{}
		result[data.MembersMDName] = members
		result[data.TypeMDName] = "package"
		result[data.NativeMDName] = false
		result[data.IsTypeMDName] = false
		result[data.ImportsMDName] = packageDef.Source
		result[data.BuiltinsMDName] = packageDef.Builtins
		result[data.SizeMDName] = members.Len()

		t := data.TypeOf(packageDef)
		if t.IsTypeDefinition() {
			result[data.TypeMDName] = t.Name()
			result[data.BasetypeMDName] = data.PackageTypeName
		}

		return data.NewStructOfTypeFromMap(ReflectReflectionType, result), nil
	}

	// Is it an Ego array datatype?
	if m, ok := source.(*data.Array); ok {
		// What is the name of the base type value? This will always
		// be an array of interface{} unless this is []byte in which
		// case the native type is []byte as well.
		btName := "[]interface{}"
		if m.Type().Kind() == data.ByteType.Kind() {
			btName = "[]byte"
		}

		// Make a list of the visible member names
		result := map[string]interface{}{
			data.SizeMDName:     m.Len(),
			data.TypeMDName:     m.TypeString(),
			data.BasetypeMDName: btName,
			data.IsTypeMDName:   false,
			data.NativeMDName:   false,
		}

		return data.NewStructOfTypeFromMap(ReflectReflectionType, result), nil
	}

	if e, ok := source.(*errors.Error); ok {
		wrappedError := e.Unwrap()

		if e.Is(errors.ErrUserDefined) {
			text := data.String(e.GetContext())

			return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
				data.TypeMDName:     "error",
				data.BasetypeMDName: "error",
				data.ErrorMDName:    wrappedError.Error(),
				data.TextMDName:     text,
				data.IsTypeMDName:   false,
				data.NativeMDName:   false,
			}), nil
		}

		return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
			data.TypeMDName:     "error",
			data.BasetypeMDName: "error",
			data.ErrorMDName:    strings.TrimPrefix(wrappedError.Error(), "error."),
			data.TextMDName:     e.Error(),
			data.ContextMDName:  data.NewStructFromMap(e.GetFullContext()),
			data.IsTypeMDName:   false,
			data.NativeMDName:   false,
		}), nil
	}

	if e, ok := source.(*errors.Error); ok {
		context := e.GetFullContext()

		return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
			data.TypeMDName:     "error",
			data.BasetypeMDName: "error",
			data.TextMDName:     e.Error(),
			data.IsTypeMDName:   false,
			data.ContextMDName:  context,
			data.NativeMDName:   true,
		}), nil
	}

	if e, ok := source.(errors.Error); ok {
		context := e.GetFullContext()

		return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
			data.TypeMDName:     "error",
			data.BasetypeMDName: "error",
			data.TextMDName:     e.Error(),
			data.IsTypeMDName:   false,
			data.ContextMDName:  context,
			data.NativeMDName:   true,
		}), nil
	}

	if e, ok := source.(error); ok {
		return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
			data.TypeMDName:     "error",
			data.BasetypeMDName: "error",
			data.TextMDName:     e.Error(),
			data.IsTypeMDName:   false,
			data.NativeMDName:   true,
		}), nil
	}

	typeValue, err := describeType(s, args)
	if err == nil {
		if t, ok := typeValue.(*data.Type); ok {
			result := map[string]interface{}{
				data.TypeMDName:     t,
				data.BasetypeMDName: t.BaseType(),
				data.IsTypeMDName:   true,
				data.SizeMDName:     data.SizeOf(source),
			}

			functionList := t.FunctionNames()
			if t.BaseType() != nil && t.BaseType().Kind() == data.InterfaceKind {
				functionList = append(functionList, t.BaseType().FunctionNames()...)
			}

			if len(functionList) > 0 {
				functions := data.NewArray(data.StringType, len(functionList))

				sort.Strings(functionList)

				for i, k := range functionList {
					var fd data.Function

					if v := t.Function(k); v != nil {
						if f, ok := v.(data.Function); ok {
							fd = f
						}
					}

					fName := fd
					name := k

					if fName.Declaration != nil {
						name = fName.Declaration.String()
					}

					_ = functions.Set(i, name)
				}

				result[data.FunctionsMDName] = functions
			}

			return data.NewStructOfTypeFromMap(ReflectReflectionType, result), nil
		}

		result := map[string]interface{}{
			data.TypeMDName:     data.String(typeValue),
			data.BasetypeMDName: "interface{}",
			data.SizeMDName:     data.SizeOf(source),
		}

		return data.NewStructOfTypeFromMap(ReflectReflectionType, result), nil
	}

	return nil, err
}

func describeBytecodeFunction(source interface{}) (interface{}, error) {
	switch v := source.(type) {
	default:
		r := reflect.ValueOf(v).MethodByName("String").Call([]reflect.Value{})
		str := r[0].Interface().(string)

		name := strings.Split(str, "(")[0]
		if name == "" {
			name = defs.Anon
		}

		size := 0
		if bc, ok := source.(*bytecode.ByteCode); ok {
			size = bc.Size()
		}

		r = reflect.ValueOf(v).MethodByName(data.DeclarationMDName).Call([]reflect.Value{})
		fd, _ := r[0].Interface().(*data.Declaration)

		return data.NewStructOfTypeFromMap(ReflectReflectionType, map[string]interface{}{
			data.TypeMDName:        funcLabel,
			data.BasetypeMDName:    funcLabel + " " + name,
			data.IsTypeMDName:      false,
			data.SizeMDName:        size,
			data.NameMDName:        name,
			data.DeclarationMDName: makeDeclaration(fd),
		}), nil
	}
}

func describeBuiltinFunction(source interface{}) (interface{}, error) {
	name := runtime.FuncForPC(reflect.ValueOf(source).Pointer()).Name()
	name = strings.Replace(name, "github.com/tucats/ego/builtins.", "", 1)
	name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)
	name = strings.Replace(name, "github.com/tucats/ego/", "", 1)

	declaration := data.GetBuiltinDeclaration(name)

	values := map[string]interface{}{
		data.TypeMDName:     builtinLabel,
		data.BasetypeMDName: builtinLabel + " " + name,
		data.IsTypeMDName:   false,
		data.NativeMDName:   true,
	}

	if declaration != nil {
		values[data.DeclarationMDName] = makeDeclaration(declaration)
	}

	return data.NewStructOfTypeFromMap(ReflectReflectionType, values), nil
}

// makeDeclaration constructs a native data structure describing a function declaration.
func makeDeclaration(fd *data.Declaration) *data.Struct {
	parameters := data.NewArray(ReflectParameterType, len(fd.Parameters))

	for n, i := range fd.Parameters {
		parameter := data.NewStruct(ReflectParameterType)
		_ = parameter.Set("Name", i.Name)
		_ = parameter.Set(data.TypeMDName, i.Type.Name())

		_ = parameters.Set(n, parameter)
	}

	returnTypes := make([]interface{}, len(fd.Returns))

	for i, t := range fd.Returns {
		returnTypes[i] = t.TypeString()
	}

	declaration := make(map[string]interface{})

	// By default, the minimum and maximum number of arguments is based
	// in the number of declared parameters.
	minArgs := len(fd.Parameters)
	maxArgs := len(fd.Parameters)

	// If it's a variadic function, the last parameter is optional and can
	// be any number of instances of that parameter. So adjust the minimum
	// to account for the fact that the last argument is optional, and set
	// the maximum to -1 to indicate there is no maximum.
	if fd.Variadic {
		minArgs = len(fd.Parameters) - 1
		maxArgs = -1
	} else {
		// If there is an explicit range of arguments where either is non-zero,
		// use that instead of the defaults.
		if fd.ArgCount[0] > 0 || fd.ArgCount[1] > 0 {
			minArgs = fd.ArgCount[0]
			maxArgs = fd.ArgCount[1]
		}
	}

	declaration["Name"] = fd.Name
	declaration["Parameters"] = parameters
	declaration["Returns"] = data.NewArrayFromInterfaces(data.StringType, returnTypes...)
	declaration["Argcount"] = data.NewArrayFromInterfaces(data.IntType, minArgs, maxArgs)

	return data.NewStructOfTypeFromMap(ReflectFunctionType, declaration)
}
