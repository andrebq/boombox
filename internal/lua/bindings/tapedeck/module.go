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
	return func(L *lua.LState) int {
		module := L.NewTable()
		L.SetFuncs(module, map[string]lua.LGFunction{
			"load": loadTapeDeck(deck),
			"list": listCassettes(deck),
		})
		L.Push(module)
		return 1
	}
}

func loadTapeDeck(d *tapedeck.D) func(*lua.LState) int {
	return func(L *lua.LState) int {
		name := L.CheckString(1)
		c := d.Get(name)
		if c == nil {
			L.RaiseError("Cassette %v not available for use", name)
		}
		L.Push(newCassetteObject(L, c))
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

func newCassetteObject(L *lua.LState, c *cassette.Control) *lua.LUserData {
	ud := L.NewUserData()
	ud.Value = c
	meta := L.NewTable()
	L.SetField(meta, "__index", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
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
	}))
	L.SetMetatable(ud, meta)
	return ud
}
