package luadefaults

import lua "github.com/yuin/gopher-lua"

// DynamicCodeLibs loads the required libs into the given *lua.LState
// dynamic code has more trust than a UserCode (which cannot load any module)
func InjectDynamicCodeLibs(L *lua.LState) {
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage}, // Must be first
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
	} {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}
}
