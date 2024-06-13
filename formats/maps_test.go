package formats

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tucats/ego/data"
)

func TestMapAsString_Homogeneous(t *testing.T) {
	m := data.NewMapFromMap(map[string]int{"apple": 10, "banana": 20})
	result := MapAsString(m, false)
	expected := `Key       Value    
======    =====    
apple     10       
banana    20       
`
	assert.Equal(t, expected, result)
}

func TestMapAsString_Heterogeneous(t *testing.T) {
	m := data.NewMapFromMap(map[string]interface{}{"apple": 10, "banana": "20"})
	result := MapAsString(m, true)
	expected := `Key       Type      Value    
======    ======    =====    
apple     int       10       
banana    string    20       
`
	assert.Equal(t, expected, result)
}

func TestMapAsString_ShowTypes(t *testing.T) {
	m := data.NewMapFromMap(map[string]int{"apple": 10, "banana": 20})
	result := MapAsString(m, true)
	expected := `Key       Type    Value    
======    ====    =====    
apple     int     10       
banana    int     20       
`
	assert.Equal(t, expected, result)
}

func TestMapAsString_EmptyMap(t *testing.T) {
	m := data.NewMapFromMap(map[string]int{})
	result := MapAsString(m, false)
	expected := `Key    Value    
===    =====    
`
	assert.Equal(t, expected, result)
}

func TestMapAsString_NilMap(t *testing.T) {
	var m *data.Map
	result := MapAsString(m, false)
	expected := ""
	assert.Equal(t, expected, result)
}
