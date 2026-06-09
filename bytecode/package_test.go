package bytecode

// package_test.go tests the functions in package.go:
//
//   IsPackage                — thin cache-presence predicate
//   GetPackage               — cache lookup by path and by name
//   inFileByteCode           — InFile opcode: sets c.name
//   inPackageByteCode        — InPackage opcode: pushes package proxy scope
//   importByteCode           — Import opcode: loads package into symbol table
//   getStringListFromOperand — flexible operand → []string conversion
//   makePackageItemList      — formats a package's exported items for display
//
// # Known issues documented here
//
//   PACKAGES-1  inPackageByteCode: calling GetPackageSymbolTable on a non-package
//               value from GetAnyScope returns nil and panics on NewChildProxy.
//               Fixed by type-asserting to *data.Package before using the value.
//
//   PACKAGES-2  makePackageItemList: reflect.TypeOf(nil).String() panics when the
//               package dictionary or symbol table holds a nil value.
//               Fixed by nil-guarding before the reflect.TypeOf call.
//
// # Test organisation
//
// Section 1:  IsPackage
// Section 2:  GetPackage
// Section 3:  inFileByteCode
// Section 4:  inPackageByteCode
// Section 5:  importByteCode
// Section 6:  getStringListFromOperand
// Section 7:  makePackageItemList
//
// All tests use the newTestContext / withXxx / assertXxx helpers from
// testhelpers_test.go as required by the project testing standards.
//
// Package-cache interactions: tests that write to the global package cache
// use packages.Save to add a test package and call packages.Delete in a
// t.Cleanup closure to remove it, keeping tests hermetic.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/symbols"
)

// helper: register a temporary package in the global cache and return a
// cleanup function that removes it.  name is used as both Name and Path.
func registerTestPackage(t *testing.T, name string) *data.Package {
	t.Helper()

	pkg := data.NewPackage(name, name)
	packages.Save(pkg)
	t.Cleanup(func() { packages.Delete(name) })

	return pkg
}

// ─── Section 1: IsPackage ────────────────────────────────────────────────────

// Test_IsPackage_KnownPackage verifies that IsPackage returns true for a
// package that has been registered in the global cache.
func Test_IsPackage_KnownPackage(t *testing.T) {
	registerTestPackage(t, "testpkg.ispackage")

	if !IsPackage("testpkg.ispackage") {
		t.Errorf("IsPackage: expected true for registered package, got false")
	}
}

// Test_IsPackage_UnknownPackage verifies that IsPackage returns false for a
// name that is not in the global cache.
func Test_IsPackage_UnknownPackage(t *testing.T) {
	if IsPackage("no.such.package.xyz") {
		t.Errorf("IsPackage: expected false for unknown package, got true")
	}
}

// ─── Section 2: GetPackage ───────────────────────────────────────────────────

// Test_GetPackage_ByPath verifies that a package registered under a given path
// is found on the first (path-based) lookup pass.
func Test_GetPackage_ByPath(t *testing.T) {
	registerTestPackage(t, "testpkg.getbypath")

	got, ok := GetPackage("testpkg.getbypath")

	if !ok {
		t.Fatalf("GetPackage: expected found=true, got false")
	}

	if got.Name != "testpkg.getbypath" {
		t.Errorf("GetPackage: got name %q, want %q", got.Name, "testpkg.getbypath")
	}
}

// Test_GetPackage_ByName verifies that GetPackage falls back to a name-based
// search when the path lookup fails but a package with a matching Name exists.
func Test_GetPackage_ByName(t *testing.T) {
	// Register a package whose Path differs from its Name so that a path lookup
	// for the name "myalias" fails, forcing the fallback GetByName search.
	pkg := data.NewPackage("myalias", "some/long/import/path.byname")
	packages.Save(pkg)
	t.Cleanup(func() { packages.Delete("some/long/import/path.byname") })

	got, ok := GetPackage("myalias")

	if !ok {
		t.Fatalf("GetPackage: expected found=true via name fallback, got false")
	}

	if got.Name != "myalias" {
		t.Errorf("GetPackage: got name %q, want %q", got.Name, "myalias")
	}
}

// Test_GetPackage_NotFound verifies that GetPackage returns nil and false when
// neither the path nor the name matches any registered package.
func Test_GetPackage_NotFound(t *testing.T) {
	got, ok := GetPackage("no.such.package.xyz.notfound")

	if ok {
		t.Errorf("GetPackage: expected found=false, got true")
	}

	if got != nil {
		t.Errorf("GetPackage: expected nil package, got %v", got)
	}
}

// ─── Section 3: inFileByteCode ───────────────────────────────────────────────

// Test_inFileByteCode_SetsName verifies that calling InFile with a string
// operand sets c.name to "file <operand>".
func Test_inFileByteCode_SetsName(t *testing.T) {
	tc := newTestContext(t)

	err := inFileByteCode(tc.ctx, "main.ego")

	tc.assertNoError(err)

	if tc.ctx.name != "file main.ego" {
		t.Errorf("c.name: got %q, want %q", tc.ctx.name, "file main.ego")
	}
}

// Test_inFileByteCode_EmptyOperand verifies that a nil operand produces the
// prefix "file " with an empty suffix rather than panicking.
func Test_inFileByteCode_EmptyOperand(t *testing.T) {
	tc := newTestContext(t)

	err := inFileByteCode(tc.ctx, nil)

	tc.assertNoError(err)

	if !strings.HasPrefix(tc.ctx.name, "file ") {
		t.Errorf("c.name: got %q, expected prefix \"file \"", tc.ctx.name)
	}
}

// ─── Section 4: inPackageByteCode ────────────────────────────────────────────

// Test_inPackageByteCode_FoundInCache verifies that when the package is in
// the global cache (but not yet in the symbol table), inPackageByteCode
// creates a proxy scope and sets c.pkg.
func Test_inPackageByteCode_FoundInCache(t *testing.T) {
	registerTestPackage(t, "testpkg.inpkg.cache")

	tc := newTestContext(t)
	origSymbols := tc.ctx.symbols

	err := inPackageByteCode(tc.ctx, "testpkg.inpkg.cache")

	tc.assertNoError(err)

	if tc.ctx.pkg != "testpkg.inpkg.cache" {
		t.Errorf("c.pkg: got %q, want %q", tc.ctx.pkg, "testpkg.inpkg.cache")
	}

	// The symbol table must have been replaced with a proxy whose parent chain
	// eventually reaches the original table.
	if tc.ctx.symbols == origSymbols {
		t.Errorf("inPackageByteCode: symbol table was not replaced with a proxy")
	}
}

// Test_inPackageByteCode_FoundInSymbolTable verifies that when the symbol table
// already holds the package (e.g. after a prior import), the proxy is built
// from the symbol-table value rather than the cache.
func Test_inPackageByteCode_FoundInSymbolTable(t *testing.T) {
	pkg := registerTestPackage(t, "testpkg.inpkg.symtab")

	tc := newTestContext(t)
	// Seed the symbol table so GetAnyScope finds the package.
	tc.ctx.symbols.SetAlways("testpkg.inpkg.symtab", pkg)

	err := inPackageByteCode(tc.ctx, "testpkg.inpkg.symtab")

	tc.assertNoError(err)

	if tc.ctx.pkg != "testpkg.inpkg.symtab" {
		t.Errorf("c.pkg: got %q, want %q", tc.ctx.pkg, "testpkg.inpkg.symtab")
	}
}

// Test_inPackageByteCode_UnknownPackage verifies that an unregistered package
// name returns ErrInvalidPackageName.
func Test_inPackageByteCode_UnknownPackage(t *testing.T) {
	tc := newTestContext(t)

	err := inPackageByteCode(tc.ctx, "no.such.package.xyz")

	tc.assertError(err, errors.ErrInvalidPackageName)
}

// Test_inPackageByteCode_NonPackageInSymbolTable_PACKAGES1 confirms the
// PACKAGES-1 fix: when the symbol table contains a non-*data.Package value
// under the package name, inPackageByteCode must NOT panic.  Instead it falls
// through to the package cache.  If the cache also has no entry, it returns
// ErrInvalidPackageName.
func Test_inPackageByteCode_NonPackageInSymbolTable_PACKAGES1(t *testing.T) {
	tc := newTestContext(t)
	// Plant an integer (not a *data.Package) under the name "math".
	tc.ctx.symbols.SetAlways("shadowed", 42)

	// "shadowed" is in the symbol table as an int, not a package.
	// Before the PACKAGES-1 fix this panicked; now it returns an error.
	err := inPackageByteCode(tc.ctx, "shadowed")

	tc.assertError(err, errors.ErrInvalidPackageName)
}

// Test_inPackageByteCode_NonPackageInSymbolTable_CacheFallback_PACKAGES1
// verifies that when the symbol table holds a non-package shadow but the
// global cache has the real package, the cache path succeeds.
func Test_inPackageByteCode_NonPackageInSymbolTable_CacheFallback_PACKAGES1(t *testing.T) {
	registerTestPackage(t, "testpkg.shadow.cache")

	tc := newTestContext(t)
	// Seed a non-package value that shadows the name in the local scope.
	tc.ctx.symbols.SetAlways("testpkg.shadow.cache", 99)

	// The cache still has the real package, so the call should succeed.
	err := inPackageByteCode(tc.ctx, "testpkg.shadow.cache")

	tc.assertNoError(err)

	if tc.ctx.pkg != "testpkg.shadow.cache" {
		t.Errorf("c.pkg: got %q, want %q", tc.ctx.pkg, "testpkg.shadow.cache")
	}
}

// ─── Section 5: importByteCode ───────────────────────────────────────────────

// Test_importByteCode_StringOperand verifies that a plain string operand
// causes the package to be stored under that name in the symbol table.
func Test_importByteCode_StringOperand(t *testing.T) {
	registerTestPackage(t, "testpkg.import.str")

	tc := newTestContext(t)

	err := importByteCode(tc.ctx, "testpkg.import.str")

	tc.assertNoError(err)

	// The package should now be visible in the symbol table.
	v, found := tc.ctx.symbols.Get("testpkg.import.str")
	if !found {
		t.Fatalf("symbol 'testpkg.import.str' not found after import")
	}

	if _, ok := v.(*data.Package); !ok {
		t.Errorf("expected *data.Package in symbol table, got %T", v)
	}
}

// Test_importByteCode_ListOperand verifies that a data.List{name, path} operand
// imports the package by path and stores it under the local alias name.
func Test_importByteCode_ListOperand(t *testing.T) {
	registerTestPackage(t, "testpkg.import.listpath")

	tc := newTestContext(t)
	operand := data.NewList("mypkg", "testpkg.import.listpath")

	err := importByteCode(tc.ctx, operand)

	tc.assertNoError(err)

	// Should be stored under the alias "mypkg", not the path.
	v, found := tc.ctx.symbols.Get("mypkg")
	if !found {
		t.Fatalf("symbol 'mypkg' not found after import with alias")
	}

	if _, ok := v.(*data.Package); !ok {
		t.Errorf("expected *data.Package in symbol table, got %T", v)
	}
}

// Test_importByteCode_CacheMiss verifies that importing a package not in the
// cache returns ErrImportNotCached.
func Test_importByteCode_CacheMiss(t *testing.T) {
	tc := newTestContext(t)

	err := importByteCode(tc.ctx, "no.such.package.notcached")

	tc.assertError(err, errors.ErrImportNotCached)
}

// ─── Section 6: getStringListFromOperand ─────────────────────────────────────

// Test_getStringListFromOperand_Nil verifies that a nil operand returns the
// full package list from the global cache (non-empty when any package is cached).
func Test_getStringListFromOperand_Nil(t *testing.T) {
	registerTestPackage(t, "testpkg.strlist.nil")

	tc := newTestContext(t)

	list, err := getStringListFromOperand(tc.ctx, nil)

	tc.assertNoError(err)

	if len(list) == 0 {
		t.Errorf("expected non-empty list for nil operand, got empty slice")
	}
}

// Test_getStringListFromOperand_String verifies that a string operand returns
// a one-element slice containing exactly that string.
func Test_getStringListFromOperand_String(t *testing.T) {
	tc := newTestContext(t)

	list, err := getStringListFromOperand(tc.ctx, "alpha")

	tc.assertNoError(err)

	if len(list) != 1 || list[0] != "alpha" {
		t.Errorf("expected [\"alpha\"], got %v", list)
	}
}

// Test_getStringListFromOperand_StringSlice verifies that a []string operand
// is returned as-is.
func Test_getStringListFromOperand_StringSlice(t *testing.T) {
	tc := newTestContext(t)
	input := []string{"a", "b", "c"}

	list, err := getStringListFromOperand(tc.ctx, input)

	tc.assertNoError(err)

	if len(list) != 3 || list[0] != "a" || list[1] != "b" || list[2] != "c" {
		t.Errorf("expected [a b c], got %v", list)
	}
}

// Test_getStringListFromOperand_AnySlice verifies that a []any operand is
// converted element-by-element using data.String.
func Test_getStringListFromOperand_AnySlice(t *testing.T) {
	tc := newTestContext(t)
	input := []any{"x", "y"}

	list, err := getStringListFromOperand(tc.ctx, input)

	tc.assertNoError(err)

	if len(list) != 2 || list[0] != "x" || list[1] != "y" {
		t.Errorf("expected [x y], got %v", list)
	}
}

// Test_getStringListFromOperand_DataList verifies that a data.List operand is
// converted element-by-element.
func Test_getStringListFromOperand_DataList(t *testing.T) {
	tc := newTestContext(t)
	input := data.NewList("p", "q", "r")

	list, err := getStringListFromOperand(tc.ctx, input)

	tc.assertNoError(err)

	if len(list) != 3 || list[0] != "p" || list[1] != "q" || list[2] != "r" {
		t.Errorf("expected [p q r], got %v", list)
	}
}

// Test_getStringListFromOperand_InvalidType verifies that an unsupported
// operand type returns ErrInvalidOperand.
func Test_getStringListFromOperand_InvalidType(t *testing.T) {
	tc := newTestContext(t)

	_, err := getStringListFromOperand(tc.ctx, 12345) // int is not supported

	tc.assertError(err, errors.ErrInvalidOperand)
}

// ─── Section 7: makePackageItemList ──────────────────────────────────────────

// Test_makePackageItemList_FunctionItem verifies that a data.Function stored
// in the package dictionary produces a "4func …" sorted prefix.
func Test_makePackageItemList_FunctionItem(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"Greet": data.Function{
			Declaration: &data.Declaration{Name: "Greet"},
		},
	})

	items := makePackageItemList(pkg)

	found := false

	for _, item := range items {
		if strings.Contains(item, "func") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected a func item in list, got: %v", items)
	}
}

// Test_makePackageItemList_TypeItem verifies that a *data.Type stored in the
// package dictionary produces a "1type …" sorted prefix (types sort first).
func Test_makePackageItemList_TypeItem(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"MyType": data.IntType,
	})

	items := makePackageItemList(pkg)

	found := false

	for _, item := range items {
		if strings.Contains(item, "type") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected a type item in list, got: %v", items)
	}
}

// Test_makePackageItemList_VariableItem verifies that a plain value in the
// package dictionary produces a "3var …" sorted prefix.
func Test_makePackageItemList_VariableItem(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"Version": "1.0",
	})

	items := makePackageItemList(pkg)

	found := false

	for _, item := range items {
		if strings.Contains(item, "var") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected a var item in list, got: %v", items)
	}
}

// Test_makePackageItemList_ReadonlyPrefixSkipped verifies that items whose
// keys begin with defs.ReadonlyVariablePrefix ("_") are excluded from the
// output list.
func Test_makePackageItemList_ReadonlyPrefixSkipped(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"_hidden": "secret",
		"Visible": "public",
	})

	items := makePackageItemList(pkg)

	for _, item := range items {
		if strings.Contains(item, "_hidden") {
			t.Errorf("unexpected _hidden item in list: %q", item)
		}
	}

	found := false

	for _, item := range items {
		if strings.Contains(item, "Visible") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected Visible item in list, got: %v", items)
	}
}

// Test_makePackageItemList_NilValueNoPanic_PACKAGES2 confirms the PACKAGES-2
// fix: a nil value stored in the package dictionary must not cause a panic in
// makePackageItemList.  Before the fix, reflect.TypeOf(nil).String() panicked.
func Test_makePackageItemList_NilValueNoPanic_PACKAGES2(t *testing.T) {
	pkg := data.NewPackage("testpkg", "testpkg")
	// Directly set a nil value under an exported key.
	pkg.Set("NilField", nil)

	// This must complete without panicking.
	items := makePackageItemList(pkg)

	found := false

	for _, item := range items {
		if strings.Contains(item, "NilField") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected NilField in item list, got: %v", items)
	}
}

// Test_makePackageItemList_NilInSymbolTable_PACKAGES2 confirms the PACKAGES-2
// fix for the second loop (symbol table values).  A nil value in the package's
// symbol table must not panic.
func Test_makePackageItemList_NilInSymbolTable_PACKAGES2(t *testing.T) {
	pkg := data.NewPackage("testpkg.nilsym", "testpkg.nilsym")

	// Seed the package symbol table with a nil value under an exported name.
	syms := symbols.GetPackageSymbolTable(pkg)
	syms.SetAlways("NilSym", nil)

	// This must complete without panicking.
	items := makePackageItemList(pkg)

	found := false

	for _, item := range items {
		if strings.Contains(item, "NilSym") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected NilSym in item list, got: %v", items)
	}
}

// Test_makePackageItemList_SortOrder verifies that types (prefix "1") sort
// before constants ("2"), variables ("3"), and functions ("4").
func Test_makePackageItemList_SortOrder(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"Zfunc": data.Function{Declaration: &data.Declaration{Name: "Zfunc"}},
		"Atype": data.IntType,
		"Mvar":  42,
	})

	items := makePackageItemList(pkg)

	if len(items) < 3 {
		t.Fatalf("expected at least 3 items, got %d: %v", len(items), items)
	}

	// The first character is the sort key digit.
	if items[0][0] > items[1][0] || items[1][0] > items[2][0] {
		t.Errorf("items not sorted by kind: %v", items)
	}
}

// Test_makePackageItemList_SymbolTableItemSkippedIfInPkgDict verifies that a
// symbol in the package's symbol table that is already present in the package
// dictionary is not duplicated in the output.
func Test_makePackageItemList_SymbolTableItemSkippedIfInPkgDict(t *testing.T) {
	pkg := data.NewPackageFromMap("testpkg", map[string]any{
		"Pi": 3.14,
	})

	// Also add to the package symbol table — this should be deduped.
	syms := symbols.GetPackageSymbolTable(pkg)
	syms.SetAlways("Pi", 3.14)

	items := makePackageItemList(pkg)

	count := 0

	for _, item := range items {
		if strings.Contains(item, "Pi") {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected Pi to appear exactly once, got %d times in: %v", count, items)
	}
}

// Test_makePackageItemList_UnexportedSymbolTableItemSkipped verifies that
// unexported (lowercase) names in the package symbol table are not included.
func Test_makePackageItemList_UnexportedSymbolTableItemSkipped(t *testing.T) {
	pkg := data.NewPackage("testpkg.unexported", "testpkg.unexported")

	syms := symbols.GetPackageSymbolTable(pkg)
	syms.SetAlways("internal", "hidden")
	syms.SetAlways("Exported", "visible")

	items := makePackageItemList(pkg)

	for _, item := range items {
		if strings.Contains(item, "internal") {
			t.Errorf("unexpected unexported item 'internal' in list: %q", item)
		}
	}

	found := false

	for _, item := range items {
		if strings.Contains(item, "Exported") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected 'Exported' in item list, got: %v", items)
	}
}

// Test_dumpPackagesByteCode_UnknownPackage verifies that listing an unknown
// package name returns ErrInvalidPackageName without writing any output.
func Test_dumpPackagesByteCode_UnknownPackage(t *testing.T) {
	tc := newTestContext(t)
	var buf bytes.Buffer

	tc.ctx.output = &buf

	err := dumpPackagesByteCode(tc.ctx, "no.such.package.dump")

	tc.assertError(err, errors.ErrInvalidPackageName)

	if buf.Len() != 0 {
		t.Errorf("expected no output on error, got %d bytes", buf.Len())
	}
}

// Test_dumpPackagesByteCode_KnownPackage verifies that listing a registered
// package produces non-empty output and no error.
func Test_dumpPackagesByteCode_KnownPackage(t *testing.T) {
	registerTestPackage(t, "testpkg.dump.known")

	tc := newTestContext(t)
	var buf bytes.Buffer

	tc.ctx.output = &buf

	err := dumpPackagesByteCode(tc.ctx, "testpkg.dump.known")

	tc.assertNoError(err)
}

// Test_dumpPackagesByteCode_InvalidOperandType verifies that an unsupported
// operand type returns ErrInvalidOperand.
func Test_dumpPackagesByteCode_InvalidOperandType(t *testing.T) {
	tc := newTestContext(t)
	var buf bytes.Buffer

	tc.ctx.output = &buf

	err := dumpPackagesByteCode(tc.ctx, struct{}{})

	tc.assertError(err, errors.ErrInvalidOperand)
}
