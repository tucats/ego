package functions

import (
	"strings"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var rasterLines = []string{
	"       ", // 0
	"*******", // 1
	"*  *  *", // 2
	" ** ** ", // 3
	" ******", // 4
	"*  *   ", // 5
	" ***** ", // 6
	"*     *", // 7
	" *   * ", // 8
	"* *   *", // 9
	" * * * ", // 10
	"   *** ", // 11
	"   *   ", // 12
	"*    * ", // 13
	"****** ", // 14
	"*      ", // 15
	"  * *  ", // 16
	"      *", // 17
	" *     ", // 18
	"  *    ", // 19
	" **    ", // 20
	"*    **", // 21
	"*  **  ", // 22
	"*  * * ", // 23
	" **   *", // 24
	" **  * ", // 25
	" *  ** ", // 26
	"*****  ", // 27
	"     * ", // 28
	"    *  ", // 29
	"**   **", // 30
	"***    ", // 31
	"    ***", // 32
	"*   * *", // 33
	"*   *  ", // 34
	"**    *", // 35
	" *    *", // 36,
	"   *  *", // 37,
	"    * *", // 38,
	" ***  *", // 39,
	". . . .",
	" . . . ",
}

var rasterDictionary map[rune][]int = map[rune][]int{
	0:   {len(rasterLines) - 2, len(rasterLines) - 1, len(rasterLines) - 2, len(rasterLines) - 1, len(rasterLines) - 2},
	' ': {0, 0, 0, 0, 0},
	'1': {0, 36, 1, 17, 0},
	'2': {36, 21, 21, 33, 39},
	'A': {4, 5, 5, 5, 4},
	'B': {1, 2, 2, 2, 3},
	'C': {6, 7, 7, 7, 8},
	'D': {1, 7, 7, 7, 6},
	'E': {1, 2, 2, 2, 2},
	'F': {1, 5, 5, 5, 5},
	'G': {6, 7, 2, 2, 11},
	'H': {1, 12, 12, 12, 1},
	'I': {7, 7, 1, 7, 7},
	'J': {13, 7, 14, 15, 15},
	'K': {1, 12, 16, 8, 7},
	'L': {1, 17, 17, 17, 17},
	'M': {1, 18, 19, 18, 1},
	'N': {1, 18, 19, 12, 1},
	'O': {6, 7, 7, 7, 6},
	'P': {1, 5, 5, 5, 20},
	'Q': {6, 7, 7, 21, 4},
	'R': {1, 5, 22, 23, 24},
	'S': {25, 2, 2, 2, 26},
	'T': {15, 15, 1, 15, 15},
	'U': {14, 17, 17, 17, 14},
	'V': {27, 28, 17, 28, 27},
	'W': {1, 28, 29, 28, 1},
	'X': {30, 16, 12, 16, 30},
	'Y': {31, 12, 32, 12, 31},
	'Z': {21, 34, 2, 9, 35},
}

func blockPrinter(s, spacing string) string {
	buffer := make([]strings.Builder, 7)

	for _, ch := range s {
		lines, ok := rasterDictionary[ch]
		if !ok {
			lines = rasterDictionary[0]
		}

		for _, line := range lines {
			r := rasterLines[line]
			for i, nib := range r {
				buffer[i].WriteRune(nib)
			}
		}

		for i := 0; i < len(buffer); i++ {
			buffer[i].WriteString(spacing)
		}
	}

	var result strings.Builder

	for _, row := range buffer {
		result.WriteString(row.String())
		result.WriteString("\n")
	}

	return result.String()
}

func blockPrint(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	msg := util.GetString(args[0])

	spacing := 2
	if len(args) > 1 {
		spacing = util.GetInt(args[1])
	}

	result := blockPrinter(msg, strings.Repeat(" ", spacing))

	return result, nil
}
