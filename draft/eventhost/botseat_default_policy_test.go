package eventhost

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

func TestEventBotSeatDefaultsToHostDefaultPolicy(t *testing.T) {
	var want string
	for _, e := range bots.Entries() {
		if e.Name != bots.Default && !bots.ReadsDecklists(e.Name) && e.MaxSeats == 0 || e.Name != bots.Default && !bots.ReadsDecklists(e.Name) && e.MaxSeats >= 2 {
			want = e.Name
			break
		}
	}
	if want == "" {
		t.Skip("no second non-Env policy registered")
	}
	h, _ := newHost(t, func(o *Options) { o.DefaultPolicy = want })
	resp := createOK(t, h, oneHumanReq())
	r := runOf(t, h, resp.Event)
	r.mu.Lock()
	defer r.mu.Unlock()
	for s := 1; s <= 3; s++ {
		if got := r.meta.Seats[s].Policy; got != want {
			t.Fatalf("seat %d policy %q, want host default %q", s, got, want)
		}
	}
	if want == bots.Default {
		t.Fatal("premise: want must differ from bots.Default")
	}
}
