package ltoj

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// ToJSONValue takes a given lua.LValue and returns an
// Go value that is know to be encodable to a JSON document
//
// The body of the code is similar to gluamapper but instead of
// using map[interface{}]interface{} it is restricted to map[string]interface{}
// and does not allow for customization options
func ToJSONValue(lv lua.LValue) interface{} {
	switch v := lv.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case *lua.LTable:
		maxn := v.MaxN()
		if maxn == 0 { // table
			ret := make(map[string]interface{})
			v.ForEach(func(key, value lua.LValue) {
				keystr := fmt.Sprint(ToJSONValue(key))
				ret[keystr] = ToJSONValue(value)
			})
			return ret
		} else { // array
			ret := make([]interface{}, 0, maxn)
			for i := 1; i <= maxn; i++ {
				ret = append(ret, ToJSONValue(v.RawGetInt(i)))
			}
			return ret
		}
	default:
		return v
	}
}
