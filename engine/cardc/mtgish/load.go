// Lifted from github.com/adams-shaun/gorge-compliance, oracle/mtgish/load.go at
// commit 285a7038 (same owner, Apache-2.0). Changes: "Vengeance" added to
// faceKeys, doc comments.

package mtgish

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// Card is one line of mtgish.lines.json: one printed card and its faces.
type Card struct {
	// Name is the card's lookup name: the single face's name, or the front
	// face's for a multi-face card.
	Name string
	// Layout is the _OracleCard variant (Card, Transforming, Split, ...).
	Layout string
	// Faces are the face nodes in printed order. Each carries Name,
	// Typeline (absent on a Room door), ManaCost, Rules, CardPT, Loyalty.
	Faces []node
	// Shared holds the fields a multi-face layout keeps at the top level
	// (a Flip card's ManaCost, a Room's Typeline).
	Shared node
}

// faceKeys are the struct fields under which a multi-face layout nests its
// faces, in printed order. TestFaceKeysMatchSchema holds them equal, as a
// set, to the schema-derived list.
var faceKeys = []string{"FrontFace", "BackFace", "Unflipped", "Flipped", "LeftDoor", "RightDoor", "Adventure", "Omen", "Prepared", "Vengeance"}

// skippedLayouts carry no printed card a gorge face corresponds to.
var skippedLayouts = map[string]bool{"StickerSheet": true, "TokenDefinition": true}

// decodeCard builds a Card from one decoded line; ok=false for a layout
// without faces.
func decodeCard(m node) (*Card, bool) {
	layout, _ := m["_OracleCard"].(string)
	if skippedLayouts[layout] {
		return nil, false
	}
	c := &Card{Layout: layout, Shared: m}
	if _, has := m["Name"]; has {
		c.Faces = append(c.Faces, m)
	}
	for _, k := range faceKeys {
		if f, isMap := m[k].(map[string]any); isMap {
			c.Faces = append(c.Faces, f)
		}
	}
	for _, f := range list(m["Cards"]) {
		if fm, isMap := f.(map[string]any); isMap {
			c.Faces = append(c.Faces, fm)
		}
	}
	if len(c.Faces) == 0 {
		return nil, false
	}
	c.Name, _ = c.Faces[0]["Name"].(string)
	return c, c.Name != ""
}

// FaceNames lists the card's face names in printed order.
func (c *Card) FaceNames() []string {
	out := make([]string, 0, len(c.Faces))
	for _, f := range c.Faces {
		n, _ := f["Name"].(string)
		out = append(out, n)
	}
	return out
}

// Load reads an mtgish.lines.json file. want, when non-nil, selects the
// cards to keep by any face name; the rest are decoded and dropped, so the
// resident set stays at the selection. Cards are keyed by Card.Name; a
// repeated name keeps its first line (the data has none at the pinned ref).
func Load(path string, want func(name string) bool) (map[string]*Card, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f, want)
}

// Read is Load over a reader.
func Read(r io.Reader, want func(name string) bool) (map[string]*Card, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	out := map[string]*Card{}
	lineNo := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			lineNo++
			var m map[string]any
			if jerr := json.Unmarshal(line, &m); jerr != nil {
				return nil, fmt.Errorf("mtgish: line %d: %v", lineNo, jerr)
			}
			if c, ok := decodeCard(m); ok && keep(c, want) {
				if _, dup := out[c.Name]; !dup {
					out[c.Name] = c
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func keep(c *Card, want func(string) bool) bool {
	if want == nil {
		return true
	}
	for _, n := range c.FaceNames() {
		if want(n) {
			return true
		}
	}
	return false
}

// SortedNames returns the keys of a Load result in sorted order.
func SortedNames(cs map[string]*Card) []string {
	out := make([]string, 0, len(cs))
	for n := range cs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
