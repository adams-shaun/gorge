package deck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// TestIsPartnerPairDoctorCompanionCorpus is the deck-layer pin for the Doctor
// Who cycle's third commander-pair family: a card carrying K:Doctor's
// companion (real corpus Rose Tyler) and a Doctor (real corpus The Tenth
// Doctor, subtype Doctor) are a legal pair, in either designation order. It
// reads the real corpus through the gitignored .cards/ tree (corpusRegistry
// below) rather than the synthetic commanderFixture, so the one production
// pair predicate (IsPartnerPair -> doctorCompanionPair) is pinned against the
// cards a real deck names, not an authored approximation of them.
//
// The engine half of the same clause already runs the real corpus through the
// shared predicate from rules/engine.go's seating gate
// (TestDoctorCompanionSeatsBothCommanders); this test closes the deck package's
// own gap, which had only synthetic fixture coverage for the ordinary Partner
// pair. It is the front-door regression pin for the da0f0448 fix ("Doctor's
// companion is a third commander-pair family") plus its follow-up corrections
// e54c1232 and 112a01b6.
func TestIsPartnerPairDoctorCompanionCorpus(t *testing.T) {
	reg := corpusRegistry(t)
	rose := corpusCard(t, reg, "Rose Tyler")
	doctor := corpusCard(t, reg, "The Tenth Doctor")

	// Preconditions: the predicate reads exactly two front-face traits, so a
	// fixture that stopped carrying either would make the assertions below
	// vacuous in one direction. Assert both are present on the FRONT face --
	// the face doctorCompanionPair actually reads (112a01b6) -- and that the
	// companion half is NOT a Doctor itself, so a predicate that required the
	// companion to carry the subtype would seat nothing.
	if !rose.Faces[0].HasKeyword("Doctor's companion") {
		t.Fatal("fixture drift: corpus Rose Tyler no longer carries K:Doctor's companion on its front face")
	}
	if frontFaceHasType(rose, "Doctor") {
		t.Fatal("fixture drift: corpus Rose Tyler unexpectedly carries the Doctor subtype; the companion half must be the non-Doctor half")
	}
	if !frontFaceHasType(doctor, "Doctor") {
		t.Fatal("fixture drift: corpus The Tenth Doctor no longer carries the Doctor subtype on its front face")
	}
	if doctor.Faces[0].HasKeyword("Doctor's companion") {
		t.Fatal("fixture drift: corpus The Tenth Doctor carries K:Doctor's companion; the intended shape is companion + plain Doctor")
	}

	// The assertion under test: the pair is legal, and the clause names roles
	// rather than positions, so both argument orders must agree.
	if !IsPartnerPair(rose, doctor) {
		t.Fatalf("IsPartnerPair(Rose Tyler, The Tenth Doctor) = false; a Doctor's-companion pair must be legal")
	}
	if !IsPartnerPair(doctor, rose) {
		t.Fatalf("IsPartnerPair(The Tenth Doctor, Rose Tyler) = false; the pair is directional-free and must be legal in either order")
	}
}

// corpusRegistry opens the repo's gitignored .cards/ corpus the way
// internal/testutil.CorpusRegistry does -- `git rev-parse --show-toplevel`
// under cards.GitEnv(), then cards.OpenCorpus -- and Skips when the corpus is
// absent. It is duplicated here rather than imported because
// internal/testutil imports this package: importing it from a deck test is an
// import cycle. A checkout with no .cards/ at all still Skips, so a clean
// clone passes.
func corpusRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	cmd := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel")
	cmd.Env = cards.GitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("corpus: could not resolve git repo root: %v", err)
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), ".cards")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("deck: no .cards/ corpus present -- run `make fetch-cards compile-cards`")
	}
	r, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatalf("corpus: could not open %s: %v", dir, err)
	}
	return r
}

// corpusCard is reg.Lookup or the test fails: a corpus card these tests depend
// on being absent is a corpus-pin change, not something to paper over.
func corpusCard(t *testing.T, reg *cards.Registry, name string) *cards.Card {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus fixture: %q missing from the registry", name)
	}
	return c
}

// frontFaceHasType reports whether c's FRONT face carries the given creature
// subtype, the same face and case-insensitive match production's
// isDoctorCard uses. Reading the front face only matters here because the
// premise being asserted is the one the predicate's face read relies on.
func frontFaceHasType(c *cards.Card, want string) bool {
	if len(c.Faces) == 0 {
		return false
	}
	for _, ty := range c.Faces[0].Types {
		if strings.EqualFold(strings.TrimSpace(ty), want) {
			return true
		}
	}
	return false
}
