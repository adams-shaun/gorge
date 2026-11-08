package eventhost

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/protocol"
)

func championReq(set string) protocol.CreateEventRequest {
	req := oneHumanReq()
	req.Set = set
	for s := 1; s <= 3; s++ {
		req.Seats[s].Drafter = "champion"
	}
	return req
}

func TestEventPersonasListChampionKeepsDefault(t *testing.T) {
	h, _ := newHost(t, nil)
	p := h.Personas()
	for _, l := range [][]string{p.Drafters, p.Builders} {
		for _, n := range []string{"champion", "rating-colour", "best23"} {
			if !slices.Contains(l, n) {
				t.Errorf("personas lack %s: %v", n, l)
			}
		}
	}
	resp := createOK(t, h, oneHumanReq())
	r := runOf(t, h, resp.Event)
	for s := 1; s <= 3; s++ {
		if got := r.meta.Seats[s].Drafter; got != "rating-colour" {
			t.Errorf("seat %d default drafter = %q", s, got)
		}
	}
	req := oneHumanReq()
	req.Seats[1].Drafter = "champion"
	resp = createOK(t, h, req)
	r = runOf(t, h, resp.Event)
	want := []string{"", "champion", "rating-colour", "rating-colour"}
	for s := 1; s <= 3; s++ {
		if got := r.meta.Seats[s].Drafter; got != want[s] {
			t.Errorf("seat %d drafter = %q, want %q", s, got, want[s])
		}
	}
}

func TestChampionPersonaFDNOnly(t *testing.T) {
	h, _ := newHost(t, nil)
	for _, set := range []string{"BLB", "TRK"} {
		_, err := h.CreateEvent(championReq(set))
		wantErr(t, err, codeBadRequest, 400)
		if !strings.Contains(err.Error(), "FDN") || !strings.Contains(err.Error(), "champion") {
			t.Errorf("%s: message %q", set, err)
		}
	}
	createOK(t, h, championReq("FDN"))
}

func TestChampionBotsDraftDeterministically(t *testing.T) {
	var first []byte
	var champPool []string
	for i := 0; i < 2; i++ {
		h, _ := newHost(t, nil)
		resp := createOK(t, h, championReq("FDN"))
		tok, _ := humanToken(t, resp)
		joinOK(t, h, resp.Event, tok, "Ann")
		pickTop(t, h, resp.Event, tok, 64)
		r := runOf(t, h, resp.Event)
		r.mu.Lock()
		b, err := json.Marshal(r.ev.Log())
		r.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first, champPool = b, poolOf(t, h, resp.Event, 1)
		} else if !bytes.Equal(first, b) {
			t.Fatal("two identical champion events diverged")
		}
	}
	h, _ := newHost(t, nil)
	resp := createOK(t, h, oneHumanReq())
	tok, _ := humanToken(t, resp)
	joinOK(t, h, resp.Event, tok, "Ann")
	pickTop(t, h, resp.Event, tok, 64)
	if reflect.DeepEqual(champPool, poolOf(t, h, resp.Event, 1)) {
		t.Fatal("champion seat 1 pool equals default persona's pool")
	}
}
