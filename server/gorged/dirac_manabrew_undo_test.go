package gorged

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host/httpapi"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

func TestDiracManaBrewLandUndoKeepsLandPlays(t *testing.T) {
	for game := 0; game < 3; game++ {
		landUndoGame(t, game, false)
	}
}

func landUndoGame(t *testing.T, game int, undo bool) {
	c := diracConfig()
	c.vsbot, c.botPolicy, c.manabrew = true, "bot", true
	base, cancel, _ := startServe(t, c)
	defer cancel()
	status, body := postGame(t, base, map[string]string{"format": "constructed", "bot_policy": "bot"})
	if status != http.StatusOK {
		t.Fatalf("create: %d %s", status, body)
	}
	var g httpapi.CreateGameResponse
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatal(err)
	}
	native := base + "/api/tables/" + g.Table + "/matches/1"
	wire := base + manabrewRoutePrefix + "/" + g.Table + "/matches/1"
	do := func(method, url string, raw []byte) (int, []byte) {
		var rd io.Reader
		if raw != nil {
			rd = bytes.NewReader(raw)
		}
		req, _ := http.NewRequest(method, url, rd)
		req.Header.Set("Authorization", "Bearer "+g.Token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, out
	}
	poll := func() (mb.GameViewDto, *mb.PromptMessage) {
		s, raw := do(http.MethodGet, wire+"/state", nil)
		if s != http.StatusOK {
			return mb.GameViewDto{}, nil
		}
		var raws []json.RawMessage
		if err := json.Unmarshal(raw, &raws); err != nil {
			t.Fatalf("state: %s", raw)
		}
		var gv mb.GameViewDto
		var pm *mb.PromptMessage
		for _, r := range raws {
			var m mb.EngineMessage
			if _, err := mb.Decode(r, &m); err != nil {
				t.Fatalf("state message %s: %v", r, err)
			}
			switch v := m.Value.(type) {
			case mb.StateUpdate:
				gv = v.GameView
			case mb.PromptMessage:
				if v.DecidingPlayerID == "player-0" {
					pm = &v
				}
			}
		}
		return gv, pm
	}
	respond := func(pm *mb.PromptMessage, out mb.PromptOutputValue) int {
		raw, err := mb.Encode(mb.ClientMessage{Value: mb.ClientResponse{Kind: "response", PromptID: pm.PromptID,
			Action: mb.PromptOutput{Type: "chooseAction", Output: mb.PromptOutputData{Value: out}}}})
		if err != nil {
			t.Fatal(err)
		}
		s, _ := do(http.MethodPost, wire+"/send", raw)
		return s
	}
	landOf := func(pm *mb.PromptMessage) string {
		if in, ok := pm.Input.Value.(mb.ChooseActionInput); ok {
			for _, a := range in.Actions {
				if a.Type == "cast" && a.Mode == "play" {
					return a.ID
				}
			}
		}
		return ""
	}
	answerNative := func() bool {
		s, raw := do(http.MethodGet, native+"/pending?seat=0", nil)
		var d struct {
			Seq    uint64 `json:"seq"`
			Player int    `json:"player"`
			Kind   string `json:"kind"`
			Min    int    `json:"min"`
			Opts   []struct {
				Index int `json:"index"`
			} `json:"options"`
		}
		if s != http.StatusOK || json.Unmarshal(raw, &d) != nil || d.Kind == "" || d.Player != 0 || d.Kind == "priority" {
			return false
		}
		choices := []int{}
		for i := 0; i < d.Min && i < len(d.Opts); i++ {
			choices = append(choices, d.Opts[i].Index)
		}
		in, _ := json.Marshal(map[string]any{"seq": d.Seq, "player": 0, "choices": choices})
		do(http.MethodPost, native+"/intent?seat=0", in)
		return true
	}
	// next is seat 0's next open chooseAction prompt.
	next := func(what string) (mb.GameViewDto, *mb.PromptMessage) {
		t.Helper()
		for at := time.Now(); time.Since(at) < 15*time.Second; time.Sleep(time.Millisecond) {
			if answerNative() {
				continue
			}
			gv, pm := poll()
			if gv.GameOver {
				return gv, nil
			}
			if pm != nil && pm.Input.Value != nil && pm.Input.Value.PromptType() == "chooseAction" {
				return gv, pm
			}
		}
		t.Fatalf("game %d: %s: no prompt", game, what)
		return mb.GameViewDto{}, nil
	}
	landInHand := func(gv mb.GameViewDto) bool {
		for _, z := range gv.Zones {
			if z.Zone != mb.ZoneHand || z.OwnerID != "player-0" {
				continue
			}
			for _, c := range z.Cards {
				if v, ok := c.Value.(mb.VisibleCard); ok && slices.Contains(v.Types, "Land") {
					return true
				}
			}
		}
		return false
	}
	playedOn := map[int]bool{}

	// The first land, the pass after it, a native undo of that pass, then a
	// restoreSnapshot of the land: the browser smoke's exact sequence.
	var landAt int64
	for {
		gv, pm := next("the first land")
		if pm == nil {
			t.Fatalf("game %d over before a land", game)
		}
		if land := landOf(pm); land != "" {
			if respond(pm, mb.ActOutput{ActionID: land}) != http.StatusNoContent {
				t.Fatalf("game %d: land refused", game)
			}
			landAt = pm.PromptID
			playedOn[gv.Turn] = true
			break
		}
		respond(pm, mb.PassOutput{})
	}
	{
		_, pm := next("after the land")
		after := pm.PromptID
		respond(pm, mb.PassOutput{})
		if s, b := do(http.MethodPost, native+"/undo?seat=0", []byte("{}")); s != http.StatusNoContent {
			t.Fatalf("undo: %d %s", s, b)
		}
		for {
			_, pm = next("after the native undo")
			if pm.PromptID == after {
				break
			}
		}
		if s := respond(pm, mb.RestoreSnapshotOutput{CheckpointID: pm.PromptID}); s != http.StatusNoContent {
			t.Fatalf("restoreSnapshot: %d", s)
		}
		for {
			_, pm = next("after restoreSnapshot")
			if pm.PromptID == landAt {
				break
			}
		}
		land := landOf(pm)
		if land == "" || respond(pm, mb.ActOutput{ActionID: land}) != http.StatusNoContent {
			t.Fatalf("game %d: the land is not offered again at %d", game, pm.PromptID)
		}
	}
	// Later turns: whenever a land waits in hand in seat 0's own main phase
	// with no land played yet, the land play is offered.
	plays := 0
	for turns := 0; plays < 3 && turns < 400; turns++ {
		gv, pm := next("later turns")
		if pm == nil {
			return
		}
		if land := landOf(pm); land != "" {
			if respond(pm, mb.ActOutput{ActionID: land}) == http.StatusNoContent {
				plays++
				playedOn[gv.Turn] = true
			}
			continue
		}
		if gv.ActivePlayerID == "player-0" && (gv.Step == mb.StepMain1 || gv.Step == mb.StepMain2) && len(gv.Stack) == 0 &&
			!playedOn[gv.Turn] && landInHand(gv) {
			t.Fatalf("game %d turn %d %s: a land waits in hand, none played this turn, and prompt %d offers no land play: %+v",
				game, gv.Turn, gv.Step, pm.PromptID, pm.Input.Value)
		}
		respond(pm, mb.PassOutput{})
	}
	t.Logf("game %d: %d later land plays", game, plays)
}
