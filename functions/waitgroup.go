package functions

import (
	"sync"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Waitgroup functions.

// sync.WaitGroup Add() function.
func waitGroupAdd(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError).In("Add()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		count := util.GetInt(args[0])
		wg.Add(count)

		return nil, nil
	}

	return nil, errors.New(errors.InvalidThisError)
}

// sync.WaitGroup Done() function.
func waitGroupDone(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ArgumentCountError).In("Done()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		wg.Done()

		return nil, nil
	}

	return nil, errors.New(errors.InvalidThisError)
}

// sync.WaitGroup Wait() function.
func waitGroupWait(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ArgumentCountError).In("Wait()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		wg.Wait()

		return nil, nil
	}

	return nil, errors.New(errors.InvalidThisError)
}
