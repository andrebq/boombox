package ltoj

import (
	"encoding/json"

	lua "github.com/yuin/gopher-lua"
)

func OpenModule() lua.LGFunction {
	return func(L *lua.LState) int {
		module := L.NewTable()
		L.SetField(module, "to_json", L.NewFunction(func(L *lua.LState) int {
			obj := ToJSONValue(L.CheckAny(1))
			buf, err := json.Marshal(obj)
			if err != nil {
				L.RaiseError("unable to encode object to JSON, %v", err)
			}
			L.Push(lua.LString(string(buf)))
			return 1
		}))
		L.SetField(module, "from_json", L.NewFunction(func(l *lua.LState) int {
			body := L.CheckString(1)
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(body), &obj)
			if err != nil {
				L.RaiseError("unable to decode object from JSON, %v", err)
			}
			L.Push(ToLuaValue(L, obj))
			return 1
		}))
		L.Push(module)
		return 1
	}
}
