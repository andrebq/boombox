package ltoj

import (
	"reflect"

	lua "github.com/yuin/gopher-lua"
)

var (
	byteSliceType    = reflect.TypeOf([]byte(nil))
	stringSliceType  = reflect.TypeOf([]string(nil))
	floatSliceType   = reflect.TypeOf([]float64(nil))
	intSliceType     = reflect.TypeOf([]int64(nil))
	booleanSliceType = reflect.TypeOf([]bool(nil))

	fastSliceCast = []reflect.Type{
		byteSliceType, stringSliceType, floatSliceType, intSliceType, booleanSliceType,
	}
)

// ToLua takes a value that represents a JSON document and turns it
// into a Lua table.
//
// Values that cannot be represented as Lua objects are set to nil,
// is is to ensure that calling:
//
// ToJSON and ToLua have a similar behavior, this function is not
// meant to be a generic mapping of any Go value to any Lua value
func ToLuaValue(L *lua.LState, val map[string]interface{}) lua.LValue {
	return toLuaMap(L, reflect.ValueOf(val))
}

func toLuaMap(L *lua.LState, val reflect.Value) *lua.LTable {
	t := L.NewTable()
	iter := val.MapRange()
	for iter.Next() {
		lk := toLuaValue(L, iter.Key())
		lv := toLuaValue(L, iter.Value())
		L.SetTable(t, lk, lv)
	}
	return t
}

func isFastSlice(val reflect.Value) bool {
	tp := val.Type()
	for _, v := range fastSliceCast {
		if v == tp {
			return true
		}
	}
	return false
}

func toLuaSlice(L *lua.LState, val reflect.Value) *lua.LTable {
	if isFastSlice(val) {
		return toLuaSliceFast(L, val)
	}
	t := L.NewTable()
	sz := val.Len()
	for i := 0; i < sz; i++ {
		L.RawSetInt(t, i+1, toLuaValue(L, val.Index(i)))
	}
	return t
}

func toLuaSliceFast(L *lua.LState, val reflect.Value) *lua.LTable {
	tp := val.Type()
	t := L.NewTable()
	switch {
	case tp == byteSliceType:
		raw := val.Interface().([]byte)
		for i, v := range raw {
			L.RawSetInt(t, i, lua.LNumber(float64(v)))
		}
	case tp == stringSliceType:
		raw := val.Interface().([]string)
		for i, v := range raw {
			L.RawSetInt(t, i, lua.LString(v))
		}
	case tp == floatSliceType:
		raw := val.Interface().([]float64)
		for i, v := range raw {
			L.RawSetInt(t, i, lua.LNumber(v))
		}
	case tp == intSliceType:
		raw := val.Interface().([]int64)
		for i, v := range raw {
			L.RawSetInt(t, i, lua.LNumber(float64(v)))
		}
	case tp == booleanSliceType:
		raw := val.Interface().([]bool)
		for i, v := range raw {
			L.RawSetInt(t, i, lua.LBool(v))
		}
	}
	return t
}

func toLuaValue(L *lua.LState, v reflect.Value) lua.LValue {
	switch v.Kind() {
	case reflect.Float64, reflect.Float32:
		return lua.LNumber(v.Float())
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		return lua.LNumber(float64(v.Int()))
	case reflect.String:
		return lua.LString(v.String())
	case reflect.Bool:
		return lua.LBool(v.Bool())
	case reflect.Map:
		return toLuaMap(L, v)
	case reflect.Slice:
		return toLuaSlice(L, v)
	case reflect.Interface:
		return toLuaValue(L, v.Elem())
	}
	return lua.LString(v.String())
}
