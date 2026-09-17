package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/internal/testutil/feedback"
)

// writeCommittedMatch takes a feedback capture's match.json bytes and
// returns the byte form safe to COMMIT: the same match.json with its
// `tokens` and `tokens_unread` fields removed. Forge token script text is
// GPL-3.0 and must never be committed, so the removed script text is
// instead SYNCED to the gitignored token directory (cmd/repro's
// testdata/.tokens/<id>/tokens.json, keyed by fixture id), so a local
// checkout can still replay that fixture with the exact historical text.
//
// Token ids the game already referenced live outside those two fields
// (a battlefield token object, a move, a stack item) survive untouched in
// the committed match.json: only the raw script-text map is stripped, and
// a replay that needs to mint one of those stems resolves its text through
// the sync directory (feedback.resolveTokens, exact text first, live
// corpus fallback).
func writeCommittedMatch(raw []byte, id string) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("repro: match.json: %w", err)
	}
	if tokRaw, ok := doc["tokens"]; ok {
		var tokens map[string]string
		if err := json.Unmarshal(tokRaw, &tokens); err != nil {
			return nil, fmt.Errorf("repro: match.json tokens: %w", err)
		}
		if len(tokens) > 0 {
			if err := syncTokens(id, tokens); err != nil {
				return nil, err
			}
		}
		delete(doc, "tokens")
	}
	delete(doc, "tokens_unread")
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("repro: re-marshal match.json: %w", err)
	}
	return out, nil
}

// syncTokens writes a capture's token script text into the gitignored
// token directory for fixture id, as one tokens.json mapping stem→text.
func syncTokens(id string, tokens map[string]string) error {
	dir, err := feedback.TokenSyncDir(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("repro: %w", err)
	}
	raw, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("repro: token sync: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tokens.json"), append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("repro: %w", err)
	}
	return nil
}
