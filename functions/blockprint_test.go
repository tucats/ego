package functions

import (
	"fmt"
	"testing"
)

func Test_blockPrinter(t *testing.T) {
	m := blockPrinter("H I J K", " ")
	fmt.Println(m)
}
