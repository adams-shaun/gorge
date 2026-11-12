package rules

// Census for the RememberPollute$ triggered abilities whose body WRITES its own
// Remembered list (a `Remember*$ True` rider such as RememberMilled$) and then
// reads it back through a `...IsRemembered` filter in the same chain. For this
// shape the trigger's fire-time capture must not count as remembered memory
// (effects.selectRemembered): the dead creature of a dies trigger is never in
// Forge's host list. A pin bump adding a card of this shape fails here and the
// card must get a rules test beside
// TestRememberMilledIsRememberedExcludesTriggerCapture.

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// rememberWritesOwn reports whether sa carries a Remember*$ True rider (the params a body uses to write its own list).
func rememberWritesOwn(sa *cards.SA) bool {
	for k, v := range sa.Params {
		if strings.HasPrefix(k, "Remember") && k != "RememberObjects" && k != "RememberTargets" &&
			strings.EqualFold(v, "True") {
			return true
		}
	}
	return false
}

func readsIsRemembered(sa *cards.SA) bool {
	for _, v := range sa.Params {
		if strings.Contains(v, ".IsRemembered") {
			return true
		}
	}
	return false
}

func captureSelectCensus(reg *cards.Registry) []string {
	set := map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, tr := range f.Triggers {
				written := false
				for sa := tr.Effect; sa != nil; sa = sa.Sub {
					if written && readsIsRemembered(sa) {
						set[f.Name] = true
					}
					if rememberWritesOwn(sa) {
						written = true
					}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func containsName(s []string, n string) bool {
	for _, x := range s {
		if x == n {
			return true
		}
	}
	return false
}

func TestRememberedCaptureSelectCensus(t *testing.T) {
	got := captureSelectCensus(testutil.CorpusRegistry(t))
	// Measured on the corpus pin: 106 cards.
	wantNames := []string{
		"Aether Rift",
		"Ainok Wayfarer",
		"Airlift Chaplain",
		"Animate Dead",
		"Arcane Proxy",
		"Avatar Destiny",
		"Barrowgoyf",
		"Boareskyr Tollkeeper",
		"Burning-Rune Demon",
		"Cackling Observer",
		"Calim, Djinn Emperor",
		"Cantankerous Keepers",
		"Capricious Hellraiser",
		"Casting of Bones",
		"Cosmic Cube",
		"Crabomination",
		"Creeping Dread",
		"Dance of the Dead",
		"Dance, Pathetic Marionette",
		"Dawnbreak Reclaimer",
		"Dazzling Sphinx",
		"Deep Space 9",
		"Distended Mindbender",
		"Dredger's Insight",
		"Etali, Primal Storm",
		"Experimental Lab",
		"Eye of the Storm",
		"Feldon, Ronom Excavator",
		"Fireglass Mentor",
		"Flourishing Bloom-Kin",
		"Frontline Heroism",
		"Getaway Barrel",
		"Gisa, Glorious Resurrector",
		"Gríma, Saruman's Footman",
		"Hellcarver Demon",
		"Herald of Amity",
		"Human-Time Lord Meta-Crisis",
		"Inquisitor Captain",
		"Invasion of Alara",
		"Jace's Mindseeker",
		"Jacob Frye",
		"Jodah, the Unifier",
		"Keen Duelist",
		"Kefka, Dancing Mad",
		"Keldon Flamesage",
		"Kiora, Sovereign of the Deep",
		"Klement, Knowledge Acolyte",
		"Knowledge Pool",
		"Last Voyage of the _____",
		"Lluwen, Imperfect Naturalist",
		"Lord of the Void",
		"Malcolm, Alluring Scoundrel",
		"Mole Module",
		"Mysterious Stranger",
		"Nashi, Searcher in the Dark",
		"Nathan Drake, Treasure Hunter",
		"Necromancy",
		"Neera, Wild Mage",
		"Omen Machine",
		"Ostrich-Horse",
		"Parker Luck",
		"Party Thrasher",
		"Plargg and Nassari",
		"Possibility Storm",
		"Priest of Possibility",
		"Rampant Frogantua",
		"Rashmi and Ragavan",
		"Rebellion of the Flamekin",
		"Rickety Gazebo",
		"Ripples of Undeath",
		"Rona, Tolarian Obliterator",
		"Rosheen, Roaring Prophet",
		"Ryan Sinclair",
		"Sanwell, Avenger Ace",
		"Screaming Nemesis",
		"Shadow Kin",
		"Shiko, Paragon of the Way",
		"Silvan Reveler",
		"Sinister Concierge",
		"Six",
		"Sonic Shrieker",
		"Soundwave, Sonic Spy",
		"Strongbox Raider",
		"Sunbird's Invocation",
		"Szarekh, the Silent King",
		"T'Chaka, Venerable King",
		"Taster of Wares",
		"Tetzin, Gnome Champion",
		"The Celestial Toymaker",
		"The Eleventh Doctor",
		"The Fifteenth Doctor",
		"The Key to the Vault",
		"Tomakul Scrapsmith",
		"Town Greeter",
		"Transwarp Hub",
		"Treasure Keeper",
		"Triumph of Saint Katherine",
		"Ultron, Artificial Malevolence",
		"Velomachus Lorehold",
		"Vhal, Scholar of Creation",
		"Vizkopa Confessor",
		"When Will You Learn?",
		"Whiskervale Forerunner",
		"Wiccan, Young Avenger",
		"Wild Evocation",
		"Wildfire Devils",
		"Wingbright Thief",
		"Your Mistake Is My Triumph",
	}
	if len(got) == 0 || !containsName(got, "Avatar Destiny") {
		t.Fatal("census precondition: the walk must reach Avatar Destiny")
	}
	if strings.Join(got, "; ") != strings.Join(wantNames, "; ") {
		t.Fatalf("census cards:\n got: %q", got)
	}
}
