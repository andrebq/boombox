package tapedeck

import (
	"context"
	"testing"
	"time"

	"github.com/andrebq/boombox/internal/lua/luadefaults"
	"github.com/andrebq/boombox/internal/testutil"
	lua "github.com/yuin/gopher-lua"
)

func TestTapedeck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	deck, cleanup := testutil.AcquirePopulatedTapedeck(ctx, t, nil)
	defer cleanup()
	L := lua.NewState(lua.Options{
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
	})
	luadefaults.InjectDynamicCodeLibs(L)
	L.SetContext(ctx)
	L.PreloadModule("tapedeck", OpenModule(deck))
	err := L.DoString(`
	local deck = require('tapedeck')
	local items = deck.list()
	if #items ~= 2 then
		error("not the expected size")
	end
	local resultset = deck.load('people'):query('select name from dataset.people')
	if #resultset.columns ~= 1 then
		error("result set should have one column, got " .. #resultset.columns)
	end
	if #resultset.rows ~= 3 then
		error("result set should have one row, got " .. #resultset.columns)
	end
	`)
	if err != nil {
		t.Fatal(err)
	}
}
