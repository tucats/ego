package validate

import "sync"

var dictionary = map[string]interface{}{}

var dictionaryLock sync.Mutex

func Lookkup(key string) interface{} {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	return dictionary[key]
}

func Define(key string, object interface{}) {
	dictionaryLock.Lock()
	defer dictionaryLock.Unlock()

	dictionary[key] = object
}
