package gorged

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDiracPoolsKeepCommander: the commander pool survives the dirac deck check, beside a
// constructed pool or alone; a pool dirac plays nothing of is empty, not a startup error,
// and a server with no playable deck at all is.
func TestDiracPoolsKeepCommander(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, err := adapter.New([]string{"fdn"}, reg, dirac.SurfaceFull)
	if err != nil {
		t.Skip(err)
	}
	dir := oneRefused(t)
	const a, b, bad = "000_FDN_top_00001_WB", "001_FDN_top_00032_WU", "dimir-tempo"

	cmd, con, refused, err := checkDiracPools(e, reg, dir, []string{a, b}, nil)
	if err != nil || len(cmd) != 2 || len(con) != 0 || len(refused) != 0 {
		t.Fatalf("commander pool alone: cmd %v con %v refused %v err %v", cmd, con, refused, err)
	}
	cmd, con, refused, err = checkDiracPools(e, reg, dir, []string{a, bad}, []string{b})
	if err != nil || len(cmd) != 1 || cmd[0] != a || len(con) != 1 || refused[bad] == "" {
		t.Fatalf("mixed pools: cmd %v con %v refused %v err %v", cmd, con, refused, err)
	}
	cmd, con, refused, err = checkDiracPools(e, reg, dir, []string{bad}, []string{a, b})
	if err != nil || len(cmd) != 0 || len(con) != 2 || refused[bad] == "" {
		t.Fatalf("unplayable commander pool: cmd %v con %v refused %v err %v", cmd, con, refused, err)
	}
	if _, _, _, err = checkDiracPools(e, reg, dir, []string{bad}, nil); err == nil || !strings.Contains(err.Error(), "no deck in") {
		t.Fatalf("nothing playable: %v", err)
	}
}
