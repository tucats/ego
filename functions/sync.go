package functions

import (
	"sync"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Mutex functions.

// sync.Mutex.Lock() function.
func mutexLock(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Lock()")
	}

	this := getNativeThis(s)
	if m, ok := this.(*sync.Mutex); ok {
		m.Lock()

		return nil, nil
	}

	return nil, errors.New(errors.ErrInvalidThis)
}

// sync.Mutex.Unlock() function.
func mutexUnlock(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Unock()")
	}

	this := getNativeThis(s)
	if m, ok := this.(*sync.Mutex); ok {
		m.Unlock()

		return nil, nil
	}

	return nil, errors.New(errors.ErrInvalidThis)
}

// Waitgroup functions.

// sync.WaitGroup Add() function.
func waitGroupAdd(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount).In("Add()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		count := datatypes.GetInt(args[0])
		wg.Add(count)

		return nil, nil
	}

	return nil, errors.New(errors.ErrInvalidThis)
}

// sync.WaitGroup Done() function.
func waitGroupDone(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Done()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		wg.Done()

		return nil, nil
	}

	return nil, errors.New(errors.ErrInvalidThis)
}

// sync.WaitGroup Wait() function.
func waitGroupWait(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Wait()")
	}

	this := getNativeThis(s)
	if wg, ok := this.(*sync.WaitGroup); ok {
		wg.Wait()

		return nil, nil
	}

	return nil, errors.New(errors.ErrInvalidThis)
}
