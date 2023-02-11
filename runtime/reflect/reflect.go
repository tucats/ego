package reflect

import (
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// describe implements the reflect.Reflect() function.
func describe(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	vv := reflect.ValueOf(args.Get(0))
	ts := vv.String()

	// If it's a builtin function, it's description will match the signature. If it's a
	// match, find out it's name and return it as a builtin.
	if ts == "<func(*symbols.SymbolTable, []interface {}) (interface {}, error) Value>" {
		name := runtime.FuncForPC(reflect.ValueOf(args.Get(0)).Pointer()).Name()
		name = strings.Replace(name, "github.com/tucats/ego/builtins.", "", 1)
		name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)
		name = strings.Replace(name, "github.com/tucats/ego/", "", 1)

		declaration := data.GetBuiltinDeclaration(name)

		values := map[string]interface{}{
			data.TypeMDName:     "builtin",
			data.BasetypeMDName: "builtin " + name,
			data.IsTypeMDName:   false,
		}

		if declaration != nil {
			values[data.DeclarationMDName] = makeDeclaration(declaration)
		}

		return data.NewStructOfTypeFromMap(reflectionType, values), nil
	}

	// If it's a bytecode.Bytecode pointer, use reflection to get the
	// Name field value and use that with the name. A function literal
	// will have no name.
	if vv.Kind() == reflect.Ptr {
		if ts == defs.ByteCodeReflectionTypeString {
			switch v := args.Get(0).(type) {
			default:
				r := reflect.ValueOf(v).MethodByName("String").Call([]reflect.Value{})
				str := r[0].Interface().(string)

				name := strings.Split(str, "(")[0]
				if name == "" {
					name = defs.Anon
				}

				r = reflect.ValueOf(v).MethodByName(data.DeclarationMDName).Call([]reflect.Value{})
				fd, _ := r[0].Interface().(*data.Declaration)

				return data.NewStructOfTypeFromMap(reflectionType, map[string]interface{}{
					data.TypeMDName:        "func",
					data.BasetypeMDName:    "func " + name,
					data.IsTypeMDName:      false,
					data.DeclarationMDName: makeDeclaration(fd),
				}), nil
			}
		}
	}

	if m, ok := args.Get(0).(data.Function); ok {
		if m.Declaration == nil {
			return data.Format(m.Value), nil
		}

		return data.NewStructOfTypeFromMap(reflectionType, map[string]interface{}{
			data.TypeMDName:        "func",
			data.BasetypeMDName:    "func " + m.Declaration.Name,
			data.IsTypeMDName:      false,
			data.DeclarationMDName: makeDeclaration(m.Declaration),
		}), nil
	}

	if s, ok := args.Get(0).(*data.Struct); ok {
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
		m[data.NativeMDName] = true
		m[data.MembersMDName] = s.FieldNamesArray(true)
		m[data.PackageMDName] = s.PackageName()

		return data.NewStructOfTypeFromMap(reflectionType, m), nil
	}

	if t, ok := args.Get(0).(*data.Type); ok {
		r := map[string]interface{}{}

		r["istype"] = true

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
			functionList = t.BaseType().FunctionNames()
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

		return data.NewStructOfTypeFromMap(reflectionType, r), nil
	}

	// Is it an Ego package?
	if m, ok := args.Get(0).(*data.Package); ok {
		// Make a list of the visible member names
		memberList := []string{}

		for _, k := range m.Keys() {
			if !strings.HasPrefix(k, data.MetadataPrefix) {
				memberList = append(memberList, k)
			}
		}

		// Sort the member list and forge it into an Ego array
		members := util.MakeSortedArray(memberList)

		result := map[string]interface{}{}
		result[data.MembersMDName] = members
		result[data.TypeMDName] = "*package"
		result[data.NativeMDName] = false
		result[data.IsTypeMDName] = false
		result[data.ImportsMDName] = m.Source
		result[data.BuiltinsMDName] = m.Builtins

		t := data.TypeOf(m)
		if t.IsTypeDefinition() {
			result[data.TypeMDName] = t.Name()
			result[data.BasetypeMDName] = data.PackageTypeName
		}

		return data.NewStructOfTypeFromMap(reflectionType, result), nil
	}

	// Is it an Ego array datatype?
	if m, ok := args.Get(0).(*data.Array); ok {
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
		}

		return data.NewStructOfTypeFromMap(reflectionType, result), nil
	}

	if e, ok := args.Get(0).(*errors.Error); ok {
		wrappedError := e.Unwrap()

		if e.Is(errors.ErrUserDefined) {
			text := data.String(e.GetContext())

			return data.NewStructOfTypeFromMap(reflectionType, map[string]interface{}{
				data.TypeMDName:     "error",
				data.BasetypeMDName: "error",
				data.ErrorMDName:    wrappedError.Error(),
				data.TextMDName:     text,
				data.IsTypeMDName:   false,
			}), nil
		}

		return data.NewStructOfTypeFromMap(reflectionType, map[string]interface{}{
			data.TypeMDName:     "error",
			data.BasetypeMDName: "error",
			data.ErrorMDName:    strings.TrimPrefix(wrappedError.Error(), "error."),
			data.TextMDName:     e.Error(),
			data.ContextMDName:  e.GetContext(),
			data.IsTypeMDName:   false,
		}), nil
	}

	if e, ok := args.Get(0).(error); ok {
		return data.NewStructOfTypeFromMap(reflectionType, map[string]interface{}{
			data.TypeMDName:     "error",
			data.BasetypeMDName: "error",
			data.TextMDName:     e.Error(),
			data.IsTypeMDName:   false,
		}), nil
	}

	typeString, err := describeType(s, args)
	if err == nil {
		result := map[string]interface{}{
			data.TypeMDName:     typeString,
			data.BasetypeMDName: typeString,
			data.IsTypeMDName:   false,
			data.SizeMDName:     data.SizeOf(args.Get(0)),
		}

		return data.NewStructOfTypeFromMap(reflectionType, result), nil
	}

	return nil, err
}

// makeDeclaration constructs a native data structure describing a function declaration.
func makeDeclaration(fd *data.Declaration) *data.Struct {
	parameters := data.NewArray(funcParmType, len(fd.Parameters))

	for n, i := range fd.Parameters {
		parameter := data.NewStruct(funcParmType)
		_ = parameter.Set("Name", i.Name)
		_ = parameter.Set(data.TypeMDName, i.Type.Name())

		_ = parameters.Set(n, parameter)
	}

	returnTypes := make([]interface{}, len(fd.Returns))

	for i, t := range fd.Returns {
		returnTypes[i] = t.TypeString()
	}

	declaration := make(map[string]interface{})

	declaration["Name"] = fd.Name
	declaration["Parameters"] = parameters
	declaration["Returns"] = data.NewArrayFromInterfaces(data.StringType, returnTypes...)
	declaration["Argcount"] = data.NewArrayFromInterfaces(data.IntType, fd.ArgCount[0], fd.ArgCount[1])

	return data.NewStructOfTypeFromMap(funcDeclType, declaration)
}
