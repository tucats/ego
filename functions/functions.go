package functions

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// FunctionDefinition is an element in the function dictionary.
type FunctionDefinition struct {
	Name      string
	Pkg       string
	Min       int
	Max       int
	ErrReturn bool
	FullScope bool
	F         interface{}
	V         interface{}
}

// MultiValueReturn is a type used to return a list of values from a builtin
// function. This an be used to return a result and an err to the caller, for
// example. The Value list must contain the values in the order received by
// the caller.
type MultiValueReturn struct {
	Value []interface{}
}

// Any is a constant that defines that a function can have as many arguments
// as desired.
const Any = math.MaxInt32

// FunctionDictionary is the dictionary of functions. As functions are determined
// to allow the return of both a value and an error as multi-part results, add the
// ErrReturn:true flag to each function definition.
var FunctionDictionary = map[string]FunctionDefinition{
	"$cast":               {Min: 2, Max: 2, F: InternalCast},
	"append":              {Min: 2, Max: Any, F: Append},
	"close":               {Min: 1, Max: 1, F: CloseAny},
	"delete":              {Min: 1, Max: 2, F: Delete, FullScope: true},
	"error":               {Min: 1, Max: 1, F: Signal},
	"index":               {Min: 2, Max: 2, F: Index},
	"len":                 {Min: 1, Max: 1, F: Length},
	"make":                {Min: 2, Max: 2, F: Make},
	"members":             {Min: 1, Max: 1, F: Members},
	"new":                 {Min: 1, Max: 1, F: New},
	"reflect":             {Min: 1, Max: 1, F: Reflect},
	"sizeof":              {Min: 1, Max: 1, F: SizeOf},
	"type":                {Min: 1, Max: 1, F: Type},
	"base64.Decode":       {Min: 1, Max: 1, F: DecodeBase64},
	"base64.Encode":       {Min: 1, Max: 1, F: EncodeBase64},
	"cipher.Create":       {Min: 1, Max: 2, F: CreateToken, ErrReturn: true},
	"cipher.Decrypt":      {Min: 2, Max: 2, F: Decrypt, ErrReturn: true},
	"cipher.Encrypt":      {Min: 2, Max: 2, F: Encrypt, ErrReturn: true},
	"cipher.Hash":         {Min: 1, Max: 1, F: Hash, ErrReturn: true},
	"cipher.Random":       {Min: 0, Max: 1, F: CipherRandom},
	"cipher.Token":        {Min: 1, Max: 2, F: Extract},
	"cipher.Validate":     {Min: 1, Max: 2, F: Validate},
	"errors.New":          {Min: 1, Max: Any, F: Signal},
	"filepath.Abs":        {Min: 1, Max: 1, F: PathAbs},
	"filepath.Base":       {Min: 1, Max: 1, F: PathBase},
	"filepath.Clean":      {Min: 1, Max: 1, F: PathClean},
	"filepath.Dir":        {Min: 1, Max: 1, F: PathDir},
	"filepath.Ext":        {Min: 1, Max: 1, F: PathExt},
	"filepath.Join":       {Min: 1, Max: Any, F: PathJoin},
	"fmt.Print":           {Min: 1, Max: Any, F: Print},
	"fmt.Printf":          {Min: 1, Max: Any, F: Printf, ErrReturn: true},
	"fmt.Println":         {Min: 0, Max: Any, F: Println},
	"fmt.Sprintf":         {Min: 1, Max: Any, F: Sprintf},
	"fmt.Sscanf":          {Min: 3, Max: Any, F: Sscanf, ErrReturn: true},
	"http.__empty":        {F: stubFunction},
	"i18n.Language":       {F: i18nLanguage},
	"i18n.T":              {Min: 1, Max: 3, F: i18nT},
	"io.Expand":           {Min: 1, Max: 2, F: Expand, ErrReturn: true},
	"io.Open":             {Min: 1, Max: 2, F: OpenFile, ErrReturn: true},
	"io.ReadDir":          {Min: 1, Max: 1, F: ReadDir, ErrReturn: true},
	"io.ReadFile":         {Min: 1, Max: 1, F: ReadFile, ErrReturn: true},
	"io.WriteFile":        {Min: 2, Max: 2, F: WriteFile, ErrReturn: true},
	"json.Unmarshal":      {Min: 1, Max: 2, F: JSONUnmarshal, ErrReturn: true},
	"json.Marshal":        {Min: 1, Max: Any, F: JSONMarshal, ErrReturn: true},
	"json.MarshalIndent":  {Min: 3, Max: 3, F: JSONMarshalIndent, ErrReturn: true},
	"math.Abs":            {Min: 1, Max: 1, F: Abs},
	"math.Log":            {Min: 1, Max: 1, F: Log},
	"math.Max":            {Min: 1, Max: Any, F: Max},
	"math.Min":            {Min: 1, Max: Any, F: Min},
	"math.Normalize":      {Min: 2, Max: 2, F: Normalize},
	"math.Random":         {Min: 1, Max: 1, F: Random},
	"math.Sqrt":           {Min: 1, Max: 1, F: Sqrt},
	"math.Sum":            {Min: 1, Max: Any, F: Sum},
	"os.Args":             {Min: 0, Max: 0, F: GetArgs, FullScope: true},
	"os.Exit":             {Min: 0, Max: 1, F: Exit},
	"os.Getenv":           {Min: 1, Max: 1, F: GetEnv},
	"os.Hostname":         {Min: 0, Max: 0, F: Hostname},
	"os.Remove":           {Min: 1, Max: 1, F: DeleteFile, ErrReturn: true},
	"profile.Delete":      {Min: 1, Max: 1, F: ProfileDelete},
	"profile.Get":         {Min: 1, Max: 1, F: ProfileGet},
	"profile.Keys":        {Min: 0, Max: 0, F: ProfileKeys},
	"profile.Set":         {Min: 1, Max: 2, F: ProfileSet},
	"sort.Sort":           {Min: 1, Max: 1, F: Sort},
	"sort.Bytes":          {Min: 1, Max: 1, F: SortBytes},
	"sort.Floats":         {Min: 1, Max: 1, F: SortFloats},
	"sort.Floatews":       {Min: 1, Max: 1, F: SortFloat32s},
	"sort.Float64s":       {Min: 1, Max: 1, F: SortFloat64s},
	"sort.Ints":           {Min: 1, Max: 1, F: SortInts},
	"sort.Int32s":         {Min: 1, Max: 1, F: SortInt32s},
	"sort.Int64s":         {Min: 1, Max: 1, F: SortInt64s},
	"sort.Strings":        {Min: 1, Max: 1, F: SortStrings},
	"strings.Blockprint":  {Min: 1, Max: 2, F: blockPrint},
	"strings.Blockfonts":  {Min: 0, Max: 0, F: blockFonts},
	"strings.Chars":       {Min: 1, Max: 1, F: Chars},
	"strings.Compare":     {Min: 2, Max: 2, F: Compare},
	"strings.Contains":    {Min: 2, Max: 2, F: Contains},
	"strings.ContainsAny": {Min: 2, Max: 2, F: ContainsAny},
	"strings.EqualFold":   {Min: 2, Max: 2, F: EqualFold},
	"strings.Fields":      {Min: 1, Max: 1, F: Fields},
	"strings.Format":      {Min: 0, Max: Any, F: Format},
	"strings.Index":       {Min: 2, Max: 2, F: Index},
	"strings.Ints":        {Min: 1, Max: 1, F: Ints},
	"strings.Join":        {Min: 2, Max: 2, F: Join},
	"strings.Left":        {Min: 2, Max: 2, F: Left},
	"strings.Length":      {Min: 1, Max: 1, F: StrLen},
	"strings.ToLower":     {Min: 1, Max: 1, F: Lower},
	"strings.Right":       {Min: 2, Max: 2, F: Right},
	"strings.Split":       {Min: 1, Max: 2, F: Split},
	"strings.String":      {Min: 1, Max: Any, F: ToString},
	"strings.Substring":   {Min: 3, Max: 3, F: Substring},
	"strings.Template":    {Min: 1, Max: 2, F: Template, ErrReturn: true},
	"strings.Tokenize":    {Min: 1, Max: 1, F: Tokenize},
	"strings.ToUpper":     {Min: 1, Max: 1, F: Upper},
	"strings.Truncate":    {Min: 2, Max: 2, F: Truncate},
	"strings.URLPattern":  {Min: 2, Max: 2, F: URLPattern},
	"sync.__empty":        {Min: 0, Max: 0, F: stubFunction}, // Package auto imports, but has no functions
	"sync.WaitGroup":      {V: sync.WaitGroup{}},
	"sync.Mutex":          {V: sync.Mutex{}},
	"time.Now":            {Min: 0, Max: 0, F: TimeNow},
	"time.Parse":          {Min: 1, Max: 2, F: TimeParse, ErrReturn: true},
	"time.reference":      {V: "Mon Jan 2 15:04:05 -0700 MST 2006"},
	"time.Since":          {Min: 1, Max: 1, F: TimeSince},
	"time.Sleep":          {Min: 1, Max: 1, F: Sleep},
	"util.Exec":           {Min: 1, Max: Any, F: Exec, ErrReturn: true},
	"util.Log":            {Min: 1, Max: 2, F: LogTail},
	"util.SetLogger":      {Min: 2, Max: 2, F: SetLogger},
	"util.Memory":         {Min: 0, Max: 0, F: MemStats},
	"util.Mode":           {Min: 0, Max: 0, F: GetMode, FullScope: true},
	"util.Packages":       {Min: 0, Max: 0, F: Packages, FullScope: true},
	"util.SymbolTable":    {Min: 0, Max: 1, F: CurrentSymbolTable, FullScope: true},
	"uuid.New":            {Min: 0, Max: 0, F: UUIDNew},
	"uuid.Nil":            {Min: 0, Max: 0, F: UUIDNil},
	"uuid.Parse":          {Min: 1, Max: 1, F: UUIDParse, ErrReturn: true},
}

// AddBuiltins adds or overrides the default function library in the symbol map.
// Function names are distinct in the map because they always have the "()"
// suffix for the key.
func AddBuiltins(symbols *symbols.SymbolTable) {
	ui.Debug(ui.CompilerLogger, "+++ Adding in builtin functions to symbol table %s", symbols.Name)

	functionNames := make([]string, 0)
	for k := range FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, n := range functionNames {
		d := FunctionDictionary[n]
		if dot := strings.Index(n, "."); dot >= 0 {
			d.Pkg = n[:dot]
			n = n[dot+1:]
		}

		if d.Pkg == "" {
			_ = symbols.SetAlways(n, d.F)
		} else {
			// Does package already exist? IF not, make it. The package
			// is just a struct containing where each member is a function
			// definition.

			pkg := datatypes.NewPackage(d.Pkg)

			if p, found := symbols.Root().Get(d.Pkg); found {
				if pp, ok := p.(datatypes.EgoPackage); ok {
					pkg = pp
				}
			} else {
				ui.Debug(ui.CompilerLogger, "    AddBuiltins creating new package %s", d.Pkg)
			}

			// Is this a value bound to the package, or a function?
			if d.V != nil {
				pkg.Set(n, d.V)

				_ = symbols.Root().SetAlways(d.Pkg, pkg)
				ui.Debug(ui.CompilerLogger, "    adding value %s to %s", n, d.Pkg)
			} else {
				pkg.Set(n, d.F)

				datatypes.SetMetadata(pkg, datatypes.TypeMDKey, datatypes.Package(d.Pkg))
				datatypes.SetMetadata(pkg, datatypes.ReadonlyMDKey, true)
				_ = symbols.Root().SetAlways(d.Pkg, pkg)

				ui.Debug(ui.CompilerLogger, "    adding builtin %s to %s", n, d.Pkg)
			}
		}
	}
}

// FindFunction returns the function definition associated with the
// provided function pointer, if one is found.
func FindFunction(f func(*symbols.SymbolTable, []interface{}) (interface{}, *errors.EgoError)) *FunctionDefinition {
	sf1 := reflect.ValueOf(f)

	for _, d := range FunctionDictionary {
		if d.F != nil { // Only function entry points have an F value
			sf2 := reflect.ValueOf(d.F)
			if sf1.Pointer() == sf2.Pointer() {
				return &d
			}
		}
	}

	return nil
}

// FindName returns the name of a function from the dictionary if one is found.
func FindName(f func(*symbols.SymbolTable, []interface{}) (interface{}, *errors.EgoError)) string {
	sf1 := reflect.ValueOf(f)

	for name, d := range FunctionDictionary {
		if d.F != nil {
			sf2 := reflect.ValueOf(d.F)
			if sf1.Pointer() == sf2.Pointer() {
				return name
			}
		}
	}

	return ""
}

func CallBuiltin(s *symbols.SymbolTable, name string, args ...interface{}) (interface{}, *errors.EgoError) {
	var fdef = FunctionDefinition{}

	found := false

	for fn, d := range FunctionDictionary {
		if fn == name {
			fdef = d
			found = true
		}
	}

	if !found {
		return nil, errors.New(errors.ErrInvalidFunctionName).Context(name)
	}

	if len(args) < fdef.Min || len(args) > fdef.Max {
		return nil, errors.New(errors.ErrPanic).Context("incorrect number of arguments")
	}

	fn, ok := fdef.F.(func(*symbols.SymbolTable, []interface{}) (interface{}, *errors.EgoError))
	if !ok {
		return nil, errors.New(errors.ErrPanic).Context(fmt.Errorf("unable to convert %#v to function pointer", fdef.F))
	}

	return fn(s, args)
}

func AddFunction(s *symbols.SymbolTable, fd FunctionDefinition) *errors.EgoError {
	// Make sure not a collision
	if _, ok := FunctionDictionary[fd.Name]; ok {
		return errors.New(errors.ErrFunctionAlreadyExists)
	}

	FunctionDictionary[fd.Name] = fd

	// Has the package already been constructed? If so, we need to add this to the package.
	if pkg, ok := s.Get(fd.Pkg); ok {
		if p, ok := pkg.(datatypes.EgoPackage); ok {
			p.Set(fd.Name, fd.F)
		}
	}

	return nil
}

func stubFunction(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return nil, errors.New(errors.ErrInvalidFunctionName)
}
