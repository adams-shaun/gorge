package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Torrent Elemental's "{3}{B/G}{B/G}: Put it onto the battlefield tapped" is
// activated from exile (CR 113.6k-style zone-restricted ability). The recording
// is the first divergence of tail-54-c001 pair 6 (autopay, mulligans 0): at
// decision 264 gorge offered activate(Torrent Elemental) at priority and the
// kernel did not.
func TestMBCTorrentElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	// A fresh-clone check exports the tree with `git archive`, so its scratch has
	// no git root: the corpus helpers cannot resolve one and synthReg would fatal
	// rather than skip. With no git root there is no corpus to replay, so skip as
	// a corpus-less checkout does.
	if _, ok := testutil.SharedCorpusDir("."); !ok {
		t.Skip("testutil: no git root, so no .cards/ corpus to replay")
	}
	stdReproNamed(t, "torrent_elemental_exile_activation")
}
