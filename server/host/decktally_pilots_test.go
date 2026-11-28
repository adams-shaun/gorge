package host

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/protocol"
)

func TestTallyDecksNamesItsPilots(t *testing.T) {
	a := fin(u8(0), "A", "B", "C", "D")
	a.BotPolicy = "bot"
	b := fin(u8(1), "A", "B")
	b.BotPolicy = "lethal-pressure"
	c := fin(nil, "A", "B", "C", "D")
	c.BotPolicy = "bot"
	got := TallyDecks([]protocol.MatchInfo{b, a, c}, 0)
	if got.MatchesCounted != 3 {
		t.Fatalf("precondition: MatchesCounted = %d, want 3", got.MatchesCounted)
	}
	if want := []string{"bot", "lethal-pressure"}; !reflect.DeepEqual(got.Pilots, want) {
		t.Errorf("Pilots = %#v, want %#v", got.Pilots, want)
	}
	if want := []int{2, 4}; !reflect.DeepEqual(got.Seats, want) {
		t.Errorf("Seats = %#v, want %#v", got.Seats, want)
	}

	human := fin(u8(0), "A", "B")
	human.BotPolicy = "bot"
	human.Seats[1].Human = true
	empty := TallyDecks([]protocol.MatchInfo{human}, 0)
	if empty.MatchesCounted != 0 {
		t.Fatalf("precondition: MatchesCounted = %d, want 0", empty.MatchesCounted)
	}
	if empty.Pilots == nil || len(empty.Pilots) != 0 || empty.Seats == nil || len(empty.Seats) != 0 {
		t.Errorf("empty tally: Pilots = %#v, Seats = %#v, want two empty non-nil slices", empty.Pilots, empty.Seats)
	}
	js, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(js); !strings.Contains(s, `"pilots":[]`) || !strings.Contains(s, `"seats":[]`) {
		t.Errorf("empty tally JSON = %s, want pilots and seats as []", s)
	}
}
