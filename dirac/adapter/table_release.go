package adapter

import (
	"runtime"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tableHold keeps a game's table in dirac's registry. A game and its forks share
// one, and the entry is released once none of them is reachable. A host has
// nothing to close: a reaped table, a finished match and a replay read once all
// release by being dropped.
type tableHold struct{ tbl *kop.Table }

// holdTable registers tbl for as long as the returned hold is reachable. tbl must
// be this hold's alone: the release takes the entry from every State on it.
func holdTable(tbl *kop.Table) *tableHold {
	dirac.Register(tbl)
	h := &tableHold{tbl: tbl}
	runtime.AddCleanup(h, dirac.ReleaseTable, tbl)
	return h
}
