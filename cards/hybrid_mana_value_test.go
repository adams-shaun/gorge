package cards

import "testing"

func TestCorpusHybridManaValues(t *testing.T) {
	r := compiledCorpus(t)

	tests := []struct {
		name      string
		want      int32
		falseWant int32
	}{
		{name: "Ognis, the Dragon's Lash", want: 4, falseWant: 5},
		{name: "Absorb", want: 3, falseWant: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			card, ok := r.Lookup(tc.name)
			if !ok || card == nil {
				t.Fatalf("corpus card %q is missing", tc.name)
			}
			if len(card.Faces) == 0 || card.Faces[0] == nil {
				t.Fatalf("corpus card %q has no front face", tc.name)
			}
			face := card.Faces[0]
			if face.ManaCost == "" {
				t.Fatalf("corpus card %q front face has no mana cost", tc.name)
			}
			gotCmc := face.Cmc()
			gotValue := face.ManaValue()
			if tc.want == tc.falseWant {
				t.Fatalf("%s precondition is vacuous: expected value equals the rejected report value", tc.name)
			}
			if gotCmc != tc.want {
				t.Errorf("%s Cmc = %d, want %d", tc.name, gotCmc, tc.want)
			}
			if gotValue != tc.want {
				t.Errorf("%s ManaValue = %d, want %d", tc.name, gotValue, tc.want)
			}
		})
	}
}
