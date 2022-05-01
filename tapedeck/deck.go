package tapedeck

import (
	"sort"

	"github.com/andrebq/boombox/cassette"
)

type (
	D struct {
		cassettes map[string]*cassette.Control
		index     string
	}
)

func New() *D {
	return &D{
		cassettes: make(map[string]*cassette.Control),
	}
}

func (d *D) Load(name string, cassette *cassette.Control) {
	if d.cassettes[name] != nil {
		d.cassettes[name].Close()
	}
	d.cassettes[name] = cassette
}

func (d *D) IndexCassette(name string) {
	d.index = name
}

func (d *D) Index() *cassette.Control {
	return d.cassettes[d.index]
}

func (d *D) List() []string {
	out := make([]string, 0, len(d.cassettes))
	for k := range d.cassettes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (d *D) Get(name string) *cassette.Control {
	return d.cassettes[name]
}

func (d *D) Close() error {
	// TODO: add some form of error list here
	for _, c := range d.cassettes {
		c.Close()
	}
	return nil
}
