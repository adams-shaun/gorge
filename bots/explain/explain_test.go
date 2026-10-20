package explain

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRowsTopByVisitsPlusChosenAndBot(t *testing.T) {
	// 20 candidates; visits descend with the index except the two extras,
	// which sit at the bottom.
	n := 20
	v := make([]int, n)
	for i := range v {
		v[i] = 100 - i
	}
	v[0] = 0  // the bot's candidate, never visited
	v[7] = 1  // the chosen one, nearly never visited
	v[3] = 99 // tie with candidate 1 (100-1 = 99): index order breaks it
	rows, dropped := Rows(n, func(i int) int { return v[i] }, 7, 0, func(i int) Row {
		return Row{Choices: []int{i}, Visits: v[i]}
	})
	if len(rows) != MaxRows+2 {
		t.Fatalf("%d rows, want %d", len(rows), MaxRows+2)
	}
	if dropped != n-len(rows) {
		t.Fatalf("dropped %d, want %d", dropped, n-len(rows))
	}
	for j := 1; j < len(rows); j++ {
		a, b := rows[j-1], rows[j]
		if a.Visits < b.Visits || (a.Visits == b.Visits && a.Choices[0] > b.Choices[0]) {
			t.Fatalf("rows not most-visited first (ties on index): %v then %v", a, b)
		}
	}
	if rows[0].Choices[0] != 1 || rows[1].Choices[0] != 3 {
		t.Fatalf("tie on 99 visits broke as %d, %d; want 1, 3", rows[0].Choices[0], rows[1].Choices[0])
	}
	var sawChosen, sawBot int
	for _, r := range rows {
		if r.Chosen {
			sawChosen++
			if r.Choices[0] != 7 {
				t.Errorf("chosen row is candidate %d", r.Choices[0])
			}
		}
		if r.Bot {
			sawBot++
			if r.Choices[0] != 0 {
				t.Errorf("bot row is candidate %d", r.Choices[0])
			}
		}
	}
	if sawChosen != 1 || sawBot != 1 {
		t.Fatalf("chosen rows %d, bot rows %d; want 1 each", sawChosen, sawBot)
	}
}

func TestRowsSmallListKeepsEverything(t *testing.T) {
	v := []int{3, 5, 0}
	rows, dropped := Rows(3, func(i int) int { return v[i] }, 1, -1, func(i int) Row { return Row{Choices: []int{i}, Visits: v[i]} })
	if dropped != 0 || len(rows) != 3 {
		t.Fatalf("%d rows, dropped %d", len(rows), dropped)
	}
	got := []int{rows[0].Choices[0], rows[1].Choices[0], rows[2].Choices[0]}
	if !reflect.DeepEqual(got, []int{1, 0, 2}) {
		t.Fatalf("order %v", got)
	}
	for _, r := range rows {
		if r.Bot {
			t.Fatal("bot < 0 marked a bot row")
		}
	}
	if !rows[0].Chosen {
		t.Fatal("chosen row unmarked")
	}
	if r, d := Rows(0, nil, 0, 0, nil); r != nil || d != 0 {
		t.Fatal("no candidates must give no rows")
	}
}

func TestUnitConversionAndRounding(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{1, 1}, {0, -1}, {0.5, 0}, {0.75, 0.5}, {0.123456, -0.753},
	} {
		if got := FromUnit(c.in); got != c.want {
			t.Errorf("FromUnit(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	if Round(0.12349) != 0.123 || Round(-0.0005) != -0.001 {
		t.Errorf("Round: %v %v", Round(0.12349), Round(-0.0005))
	}
}

// TestNoteJSONShape pins the seam's field names (the client reads them).
func TestNoteJSONShape(t *testing.T) {
	n := Note{Seat: 1, Policy: "az-redeal", Source: Policy, Bot: []int{2},
		Search: &Search{Engine: "azmcts", Sims: 4, RootValue: 0.25, Rows: []Row{{Choices: []int{2}, Visits: 3, Prior: 0.5, Q: 0.1, Chosen: true, Bot: true}}, Dropped: 1},
		Net:    &Net{Value: -0.5, Scores: []float64{1, 2}}}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"seat":1,"policy":"az-redeal","source":"policy","bot":[2],"search":{"engine":"azmcts","sims":4,"root_value":0.25,"rows":[{"choices":[2],"visits":3,"prior":0.5,"q":0.1,"chosen":true,"bot":true}],"dropped":1},"net":{"value":-0.5,"scores":[1,2]}}`
	if string(raw) != want {
		t.Fatalf("got  %s\nwant %s", raw, want)
	}
	raw, _ = json.Marshal(Note{Seat: 0, Source: RefusalFallback, Rung: 2, Reason: "x"})
	if string(raw) != `{"seat":0,"source":"refusal_fallback","reason":"x","rung":2}` {
		t.Fatalf("driver note %s", raw)
	}
}
