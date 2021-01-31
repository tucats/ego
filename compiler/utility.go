package compiler

import (
	"github.com/google/uuid"
)

// MakeSymbol creates a unique symbol name for use
// as a temporary variable, etc. during compilation.
func MakeSymbol() string {
	x := uuid.New().String()

	return "#" + x[28:]
}
