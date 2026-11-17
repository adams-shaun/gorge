package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 613.1f, 602.2: Lavamancer's Skill grants "{T}: deals 1 damage to target
// creature" to the enchanted creature and, if that creature is a Wizard, a
// second, different granted ability ("{T}: deals 2 damage"). A Wizard
// carrying it has two activations to offer at priority.
func TestMBCLavamancerSSkillReprosLockstep(t *testing.T) {
	// A source archive with no .git (the precheck's fresh tree) has no corpus
	// either: CorpusDirOrSkip fails when git can't resolve a repository, so
	// skip here first.
	if _, ok := testutil.SharedCorpusDir("."); !ok {
		t.Skip("no git repository, so no .cards corpus")
	}
	t.Parallel()
	stdReproNamed(t, "enchanted_wizard_two_granted_abilities")
}
