package functions

import (
	"sort"
	"strings"

	"github.com/common-nighthawk/go-figure"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

var fontSet []string

func BlockPrint(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	initFonts()

	msg := data.String(args[0])

	fontName := "standard"
	if len(args) > 1 {
		fontName = data.String(args[1])
	}

	if !isFont(fontName) {
		return nil, errors.ErrNoSuchAsset.Context(fontName)
	}

	myFigure := figure.NewFigure(msg, fontName, true)

	return myFigure.String(), nil
}

func BlockFonts(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	initFonts()

	result := data.NewArray(data.StringType, len(fontSet))

	for idx, name := range fontSet {
		_ = result.Set(idx, name)
	}

	return result, nil
}

func initFonts() {
	if fontSet == nil {
		fontSet = figure.AssetNames()
		sort.Strings(fontSet)

		for idx := 0; idx < len(fontSet); idx++ {
			fontSet[idx] = strings.TrimPrefix(strings.TrimSuffix(fontSet[idx], ".flf"), "fonts/")
		}
	}
}

func isFont(name string) bool {
	for _, font := range fontSet {
		if strings.EqualFold(name, font) {
			return true
		}
	}

	return false
}
