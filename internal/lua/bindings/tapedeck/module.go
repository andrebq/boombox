package tapedeck

import (
	"bytes"
	"encoding/json"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/logutil"
	"github.com/andrebq/boombox/internal/lua/ltoj"
	"github.com/andrebq/boombox/tapedeck"
	lua "github.com/yuin/gopher-lua"
)

func OpenModule(deck *tapedeck.D) func(*lua.LState) int {
	return openModule(deck, false)
}

func OpenPrivilegedModule(deck *tapedeck.D) func(*lua.LState) int {
	return openModule(deck, true)
}

func openModule(deck *tapedeck.D, moduleHasPrivileges bool) func(*lua.LState) int {
	return func(L *lua.LState) int {
		module := L.NewTable()
		L.SetFuncs(module, map[string]lua.LGFunction{
			"load": loadTapeDeck(deck, moduleHasPrivileges),
			"list": listCassettes(deck),
		})
		L.Push(module)
		return 1
	}
}

func loadTapeDeck(d *tapedeck.D, moduleHasPrivileges bool) func(*lua.LState) int {
	return func(L *lua.LState) int {
		name := L.CheckString(1)
		c := d.Get(name)
		if c == nil {
			L.RaiseError("Cassette %v not available for use", name)
		}
		if !c.Queryable() {
			// this means the cassette could be writable by the module
			// so now we need to check that the cassette is configured
			// to allow extended privileges
			// as well as check if the tapedeck module is configured
			// with extended privileges
			if !(c.HasPrivileges() && moduleHasPrivileges) {
				L.RaiseError("Cannot load cassette %v, module requires privileges but casset does not have them", name)
			}
		}
		L.Push(newCassetteObject(L, c, moduleHasPrivileges))
		return 1
	}
}

func listCassettes(d *tapedeck.D) func(*lua.LState) int {
	return func(L *lua.LState) int {
		items := d.List()
		t := L.NewTable()
		for i, v := range items {
			L.RawSetInt(t, i+1, lua.LString(v))
		}
		L.Push(t)
		return 1
	}
}

func newCassetteObject(L *lua.LState, c *cassette.Control, moduleHasPrivileges bool) *lua.LUserData {
	ud := L.NewUserData()
	ud.Value = c
	meta := L.NewTable()
	funcTbl := map[string]lua.LGFunction{
		"query": func(L *lua.LState) int {
			ctx := L.Context()
			if ctx == nil {
				L.RaiseError("Cannot perform a cassette query without a context object!")
			}
			if _, ok := ctx.Deadline(); !ok {
				L.RaiseError("Cannot perform a cassette query from a lua.LState not bound to a context with deadline!")
			}
			log := logutil.GetOrDefault(ctx)
			if !c.Queryable() {
				L.RaiseError("Cassette is not queryable")
			}
			ud := L.CheckUserData(1)
			if ud.Value != c {
				L.RaiseError("Invalid condition, cannot query one cassette using another one as parameter!")
			}
			sql := L.CheckString(2)
			var args []interface{}
			for i := 3; i < L.GetTop(); i++ {
				args = append(args, L.CheckString(i))
			}
			var out bytes.Buffer
			// use the default max size
			// TODO: this is unsafe, because callers could call Query multiple times and load more than 1MB of data
			// While this might be safe for dynamic code loaded from a cassette (the cassette is more trustworthy than user-code)
			// this function should never be used with untrusted code (aka user input!)
			err := c.Query(ctx, &out, -1, sql, args...)
			if err != nil {
				// TODO: this log should be sampled
				log.Warn().Err(err).Str("sql", sql).Msg("Unable to perform query against cassette")
				L.RaiseError("Unable to run query against cassette, check logs for more information")
			}
			var obj map[string]interface{}
			err = json.Unmarshal(out.Bytes(), &obj)
			if err != nil {
				log.Error().Err(err).Msg("This should neven happen, but a cassette query could not be decoded as JSON")
				L.RaiseError("Unable to decode query results to an appropriate representation")
			}
			L.Push(ltoj.ToLuaValue(L, obj))
			return 1
		},
	}
	if moduleHasPrivileges && c.HasPrivileges() {
		funcTbl["unsafe_query"] = func(L *lua.LState) int {
			// another huge method!
			ctx := L.Context()
			if ctx == nil {
				L.RaiseError("Cannot perform a cassette query without a context object!")
			}
			if _, ok := ctx.Deadline(); !ok {
				L.RaiseError("Cannot perform a cassette query from a lua.LState not bound to a context with deadline!")
			}
			log := logutil.GetOrDefault(ctx)
			ud := L.CheckUserData(1)
			if ud.Value != c {
				L.RaiseError("Invalid condition, cannot query one cassette using another one as parameter!")
			}
			sql := L.CheckString(2)
			var args []interface{}
			hasOutput := L.CheckBool(3)
			for i := 4; i < L.GetTop(); i++ {
				args = append(args, L.CheckString(i))
			}
			resultSet, err := c.UnsafeQuery(ctx, sql, hasOutput, args...)
			if err != nil {
				log.Error().Err(err).Str("sql", sql).Msg("Unable to perform unsafe query")
				L.RaiseError("Unable to query cassette")
			}
			if !hasOutput {
				L.Push(L.NewTable())
				return 1
			}
			luaset := L.NewTable()
			for i, v := range resultSet {
				L.RawSetInt(luaset, i+1, ltoj.ToLuaArray(L, v))
			}
			L.Push(luaset)
			return 1
		}
	}
	L.SetField(meta, "__index", L.SetFuncs(L.NewTable(), funcTbl))
	L.SetMetatable(ud, meta)
	return ud
}
