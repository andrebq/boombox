package ltoj

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
)

func TestToFromJSON(t *testing.T) {
	initialJSON := `{"ddl":{"create":"create table if not exists wind(utc_timestamp text,wind_generation_actual real,wind_capacity real,temperature real)"},"importedFromFile":"germany-wind-energy.csv","license":"CC0: Public Domain","originalSource":"https://open-power-system-data.org/","url":"https://www.kaggle.com/datasets/aymanlafaz/wind-energy-germany"}`

	var initialMap map[string]interface{}
	var err error
	err = json.Unmarshal([]byte(initialJSON), &initialMap)
	if err != nil {
		t.Fatal(err)
	}
	l := lua.NewState(lua.Options{SkipOpenLibs: true})
	val := ToLuaValue(l, initialMap)
	if val == nil {
		t.Fatal("Should have processed the value")
	}
	decodedMap := ToJSONValue(val)
	if decodedMap == nil {
		t.Fatal("Should have decoded the map")
	}
	buf, err := json.Marshal(decodedMap)
	if err != nil {
		t.Fatal(err)
	} else {
		assert.JSONEq(t, initialJSON, string(buf))
	}
	if !reflect.DeepEqual(decodedMap, initialMap) {
		t.Fatal("Lossy conversion")
	}
}
