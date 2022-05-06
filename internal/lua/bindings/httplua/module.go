package httplua

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/andrebq/boombox/internal/lua/ltoj"
	"github.com/julienschmidt/httprouter"
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
	meta := L.NewTable()
	var parsedBody lua.LValue
	indexTbl := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"param": func(L *lua.LState) int {
			name := L.CheckString(1)
			err := req.ParseForm()
			if err != nil {
				L.Push(lua.LNil)
				return 1
			}
			val := req.FormValue(name)
			L.Push(lua.LString(val))
			return 1
		},
		"route_param": func(L *lua.LState) int {
			name := L.CheckString(1)
			param := httprouter.ParamsFromContext(req.Context()).ByName(name)
			L.Push(lua.LString(param))
			return 1
		},
		"parse_body": func(L *lua.LState) int {
			if parsedBody != nil {
				L.Push(parsedBody)
				return 1
			}
			codec := "json"
			if L.GetTop() >= 1 {
				codec := L.CheckString(1)
				switch codec {
				case "json", "", "application/json":
					codec = "json"
				case "text", "text/plain":
					codec = "text"
				}
			}
			switch codec {
			case "text":
				buf, err := ioutil.ReadAll(req.Body)
				if err != nil {
					L.RaiseError("unable to read request body")
				}
				// TODO: is checking for utf-8 correctness a good idea here?
				parsedBody = lua.LString(string(buf))
				L.Push(parsedBody)
				return 1
			case "json":
				buf, err := ioutil.ReadAll(req.Body)
				if err != nil {
					L.RaiseError("unable to read request body")
				}
				var val map[string]interface{}
				err = json.Unmarshal(buf, &val)
				if err != nil {
					L.RaiseError("unable to parse request body as JSON")
				}
				parsedBody = ltoj.ToLuaValue(L, val)
				L.Push(parsedBody)
				return 1
			}
			L.RaiseError("codec %v is not supported", codec)
			return 0
		},
	})
	L.SetField(meta, "__index", indexTbl)
	L.SetMetatable(ud, meta)
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
