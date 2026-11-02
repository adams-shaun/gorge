package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An ability keeps its source's name when the source is a token that left for a library:
// the token has ceased to exist (CR 111.8), which no library hides.
func TestSourceHiddenInALibraryUnlessACeasedToken(t *testing.T) {
	if !sourceHidden(&dirac.ProjObj{Zone: kop.ZoneLibrary}, 0) {
		t.Error("a card in a library is not hidden")
	}
	if sourceHidden(&dirac.ProjObj{Zone: kop.ZoneLibrary, Token: true}, 0) {
		t.Error("a ceased token in a library is hidden")
	}
}
