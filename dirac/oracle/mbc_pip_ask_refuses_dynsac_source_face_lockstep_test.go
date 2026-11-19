package oracle

import (
	"testing"
)

// CR 601.2b with CR 601.2g: a hybrid pip's colour ask offers a face only when
// the whole cost stays payable, and the probe reads the same sources the
// payment window can activate — a dynamic {T}, sacrifice source (Lotus
// Blossom's "{T}, Sacrifice Lotus Blossom: Add X mana of any one color")
// included, its petals' mana a face's completion. Nature's Chant ({1}{G/W})
// with two floated {W} and an untapped Lotus Blossom at one petal: the {G}
// face is payable (tap and sacrifice the Lotus for {G}, a floated {W} pays
// the {1}), so gorge offers choose:pay W and G; the kernel priced the face
// against a census that leaves the dynamic source out and offered only W
// (decision 92 of tail-74-c001 pair 25, options options).
func TestMBCNatureSChantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pip_ask_refuses_dynsac_source_face")
}
