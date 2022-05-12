package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/tapedeck"
	lua "github.com/yuin/gopher-lua"
)

type (
	ToHandler func(context.Context, *cassette.Control, lua.LGFunction) (http.Handler, error)
)

func AsHandler(ctx context.Context, d *tapedeck.D, tapedeckModule lua.LGFunction, cassetteToHandler ToHandler) (http.Handler, error) {
	cassettes := d.List()
	mux := http.NewServeMux()
	for _, c := range cassettes {
		prefix := fmt.Sprintf("/%v", c)
		handler, err := cassetteToHandler(ctx, d.Get(c), tapedeckModule)
		if err != nil {
			return nil, err
		}
		mux.Handle(fmt.Sprintf("%v/", prefix), http.StripPrefix(prefix, handler))
	}
	idx := d.Index()
	if idx != nil {
		idxHandler, err := cassetteToHandler(ctx, idx, tapedeckModule)
		if err != nil {
			return nil, err
		}
		mux.Handle("/", idxHandler)
	}
	return mux, nil
}
