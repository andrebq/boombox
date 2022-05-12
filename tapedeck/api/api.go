package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/tapedeck"
)

type (
	ToHandler func(context.Context, *cassette.Control) (http.Handler, error)
)

func AsHandler(ctx context.Context, d *tapedeck.D, cassetteToHandler ToHandler) (http.Handler, error) {
	cassettes := d.List()
	mux := http.NewServeMux()
	for _, c := range cassettes {
		prefix := fmt.Sprintf("/%v", c)
		handler, err := cassetteToHandler(ctx, d.Get(c))
		if err != nil {
			return nil, err
		}
		mux.Handle(fmt.Sprintf("%v/", prefix), http.StripPrefix(prefix, handler))
	}
	idx := d.Index()
	if idx != nil {
		idxHandler, err := cassetteToHandler(ctx, idx)
		if err != nil {
			return nil, err
		}
		mux.Handle("/", idxHandler)
	}
	return mux, nil
}
