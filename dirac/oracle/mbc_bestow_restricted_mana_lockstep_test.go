package oracle

import "testing"

// CR 106.6 with CR 702.103a: Battery Bearer grants "{T}: Add {C}. This mana
// can't be spent to cast a nonartifact spell" to the controller's untapped
// creatures, and that mana cannot pay Nyxborn Wolf's bestow cost {4}{G}. The
// printed sources leave only three mana, so gorge offers no bestowed cast
// while the kernel counted the creatures' restricted mana toward it
// (tail-27-c000 pair 38, decision 533).
func TestMBCNyxbornWolfReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_restricted_granted_mana")
}

// CR 702.103b: a card cast for its bestow cost is an Aura spell, not a
// creature spell, so mana that may be spent only on creature spells (Gwenna,
// Eyes of Gaea's {T}) cannot pay the bestow cost. The player's only black
// source was that Gwenna, and gorge offered the bestowed cast of Nyxborn
// Eidolon priced as a creature card in hand (decision 158 of the recording;
// dirac, which reads the bestowed spell as an Aura, withheld it).
func TestMBCNyxbornEidolonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_cast_restricted_mana_reads_an_aura_spell")
}
