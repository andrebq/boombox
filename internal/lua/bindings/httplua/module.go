package httplua

import (
	"io"
	"net/http"

	lua "github.com/yuin/gopher-lua"
)

func OpenServer(w http.ResponseWriter, req *http.Request) lua.LGFunction {
	return func(L *lua.LState) int {
		module := L.NewTable()
		L.SetField(module, "req", reqToLua(L, req))
		L.SetField(module, "res", resToLua(L, w))
		L.Push(module)
		return 1
	}
}

func reqToLua(L *lua.LState, req *http.Request) lua.LValue {
	ud := L.NewUserData()
	ud.Value = req
	return ud
}

func resToLua(L *lua.LState, res http.ResponseWriter) lua.LValue {
	ud := L.NewUserData()
	ud.Value = res
	meta := L.NewTable()
	L.SetField(meta, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"write_body": func(L *lua.LState) int {
			actualRes := L.CheckUserData(1)
			if actualRes.Value != res {
				L.RaiseError("gotcha! trying to sneaky calls to a different http response object!")
			}
			body := L.CheckString(2)
			n, err := io.WriteString(res, body)
			if err != nil {
				L.RaiseError("unable to write body to client, check logs for more information")
			}
			L.Push(lua.LNumber(float64(n)))
			return 1
		},
		"write_status": func(l *lua.LState) int {
			status := L.CheckInt(1)
			res.WriteHeader(status)
			L.Push(lua.LTrue)
			return 1
		},
	}))
	L.SetMetatable(ud, meta)
	return ud
}
