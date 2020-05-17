package main

import (
	"errors"
)

// FunctionPi implements the pi() function
func FunctionPi(args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.New("too many arguments to pi()")
	}
	return 3.1415926535, nil
}
