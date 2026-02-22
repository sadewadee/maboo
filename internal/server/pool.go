package server

import (
	"github.com/sadewadee/maboo/internal/phpengine"
	"github.com/sadewadee/maboo/internal/worker"
)

// Pool is the interface for worker pools.
type Pool interface {
	Start() error
	Stop() error
	Exec(ctx *phpengine.Context, script string) (*phpengine.Response, error)
	Mode() string
	Stats() worker.StatsGetter
}
