package env_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// protocolV2SHA256 pins the Protocol v2 conformance corpus the port vendors
// from SpellBench. The corpus is an external specification: env cannot replay
// it (its rows are host/agent traffic around a fake engine), so byte identity
// is the contract. Any edited or added transcript must update this manifest
// deliberately.
var protocolV2SHA256 = map[string]string{
	"agent_error_decision_pending.transcript.jsonl":              "ea54c44f55f1200c52a48a9397a07fcb579f776c55b3f28661a7462b32c4795b",
	"agent_error_game_already_active.transcript.jsonl":           "291a55d03145512370dbd0513f00f69f2408e32fb85348c7370957c40491dc6a",
	"agent_error_internal_error.transcript.jsonl":                "edc02793355df0dbbf3c128f8a716b0d21384092f28c999c6300ffe2bf51b64d",
	"agent_error_malformed_json.transcript.jsonl":                "b43d62eef4c4983dd66cd7fd06a8c1936751fa7b7a81c3485f520f9837fa2710",
	"agent_error_malformed_request.transcript.jsonl":             "6f1181259ab11ad3e7d829bf8f8ac22fb3a55c943a27248e7d68e56956847b0d",
	"agent_error_protocol_mismatch.transcript.jsonl":             "4768e7f54d989df5d6f75ea8d49c8331925b4c8acb84a92f2fd35a02a851907f",
	"agent_error_unknown_game.transcript.jsonl":                  "65353bd31ff96adff64e581a7913e162b0144b06be9c3b2b819df5897bc18888",
	"engine_error_candidate_id_out_of_range.transcript.jsonl":    "af8eeddbc06fc59eaa55acb213b95cad756368d51019a5d15985bfbe9e2ffa48",
	"engine_error_deck_id_mismatch.transcript.jsonl":             "88af8d447398e77f7bb849aadc1886d20258e83ca9fd55baf06cd23842f93a16",
	"engine_error_expected_step_mismatch.transcript.jsonl":       "ac1bdd6faa5c3aa851556e6994b9831d2bc064050a8bff6334b161d2cfd4ce8d",
	"engine_error_game_already_active.transcript.jsonl":          "36c144ac5751cf93d37e1c2b0b73988a44fc522b79e5533aa64e9e85ebe68f16",
	"engine_error_game_already_terminal.transcript.jsonl":        "e508055babfc21e02f8bb5c72f72b53c546333efb4c7b1411d22a52adf47d58a",
	"engine_error_game_id_mismatch.transcript.jsonl":             "277e89e9d03dcf048171560dc9050b8d49b072b9bc11aad845a64beb3d4bce2e",
	"engine_error_malformed_json.transcript.jsonl":               "cb91c08e8162ddd0655cdf3da4c9fe39b1bde4c04a5c1725455961fefefa98b1",
	"engine_error_malformed_request.transcript.jsonl":            "340cbd1df36029becde66ce39745319b3bf75a6c7c68b9c5439da3286bc3c5cb",
	"engine_error_malformed_request_non_object.transcript.jsonl": "c13969fbf68829fde2d7890742fd5491a99aa197dfc755f28f168d3b26b10d72",
	"engine_error_probe_refused.transcript.jsonl":                "1a9898a2b2e79fe3682b8415b4eabfe5eb67d9e905bbe98f86c61154bab99f65",
	"engine_error_protocol_mismatch.transcript.jsonl":            "660825b048f352ef1d9c52634a330929779fe8cf4b068a6108741340697edf33",
	"engine_error_request_id_reuse_mismatch.transcript.jsonl":    "decada70c9aad2d7320089d24d1ef47f3ba5f0c2e5ee1cd639692fa9e40b1ea8",
	"engine_error_semantic_echo_mismatch.transcript.jsonl":       "9b71c29db89f956ec768051b520974552b5571d193ed9a11b441107a551a7761",
	"engine_error_step_before_reset.transcript.jsonl":            "8d6e5d0245ee1823547008b9e3d6b26c41fe592cc2be0209a1c5643e73902b0e",
	"engine_error_unsupported_deck.transcript.jsonl":             "2d69828ad8e6523a3c3b40b702d119bf4297111021a4ce3203e1815285cf209a",
	"engine_error_unsupported_format.transcript.jsonl":           "7c301bf3f53a84fb3813466c49f492db4e3e2d87140a8634e123878d4aa32789",
	"engine_error_unsupported_request.transcript.jsonl":          "3d66fe0fe292befed8cbed5891dc6e9857cddb62b3d5fcbebacd1e127fbe2c25",
	"engine_error_unsupported_rule.transcript.jsonl":             "487e030412e6c73d6a4019f87611c44007922714f40eb6fcc6dc4dad86d31f91",
	"engine_validate_deck.transcript.jsonl":                      "11a2274b09fc4b25f7b3f3dd478d24a8b01b55c0762232bfda3d17c4d47996eb",
	"game_board_tour.transcript.jsonl":                           "0df281b8a0aa0370a07c1213ea2cf0e1e73eb2438f2815acf9f523cc7837e138",
	"game_forfeit_invalid_selection.transcript.jsonl":            "ce49ff7b9c2eb7659828ff7c0d159a74d6e93e2e6a0150f1d178c9b5a2ce8e83",
	"game_halt_host_validator_v4.transcript.jsonl":               "5e6d62a0e59138064d2dfbf1e7e0b8d36779879dae843fb72b9b3423b0ffbc28",
	"game_kinds_tour.transcript.jsonl":                           "a13fb32cececd2157447868ad3789ac76476e657aae9021490d3c8328f07f844",
	"game_knowledge_tour.transcript.jsonl":                       "c5f7323d0edef96098ebdded0984e2d5479b5c95d1265d5f2712736a469c0dc0",
	"game_mandatory_loop.transcript.jsonl":                       "28af32389887cb9f2714d33c90ae55da9b1240993bb3586daa00aaad88e267f3",
	"game_scoring.transcript.jsonl":                              "48f5847081ba82c6117defab7187356729eb9b8fdc5e78ab9d8d83eb74c092bb",
	"game_stalling_forfeit.transcript.jsonl":                     "64e85dc0cce8ea03e4896e8ec2c99c9ea76f0cb26d8301beac546c05cd9254bf",
	"index.json":                                                 "ddadd5d86f2c8baa80fead19223dd6c9158926df4e56f4f52f3e693db532344d",
	"test_vectors.json":                                          "9573b55cdd79438486846acb2c43cab705b99cfd926955f5bd8d456eac09a55b",
}

// checkProtocolV2Corpus is the replayable half of the vendored-corpus check:
// env cannot execute these transcripts (they surround a fake engine), so it
// proves the bytes are exactly the pinned upstream corpus and that the corpus
// is internally well formed.
func checkProtocolV2Corpus(t *testing.T) {
	t.Helper()
	dir := filepath.Join("..", "testdata", "goldens", "protocol_v2")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("%s: unexpected directory %s", dir, entry.Name())
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) != len(protocolV2SHA256) {
		t.Fatalf("%s: %d files, manifest has %d", dir, len(files), len(protocolV2SHA256))
	}
	for _, name := range files {
		want, ok := protocolV2SHA256[name]
		if !ok {
			t.Fatalf("%s: unexpected file %s", dir, name)
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
			t.Fatalf("%s: sha256 %s, want %s", name, got, want)
		}
	}

	var index struct {
		Schema      string                     `json:"schema"`
		Transcripts map[string]json.RawMessage `json:"transcripts"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "index.json")); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(b, &index); err != nil {
		t.Fatalf("index.json: %v", err)
	}
	if index.Schema != "spellbench-goldens/v2" {
		t.Fatalf("index.json schema %q", index.Schema)
	}
	if len(index.Transcripts) != 34 {
		t.Fatalf("index.json: %d transcripts, want 34", len(index.Transcripts))
	}
	transcripts := 0
	for _, name := range files {
		if !strings.HasSuffix(name, ".transcript.jsonl") {
			continue
		}
		transcripts++
		if _, ok := index.Transcripts[name]; !ok {
			t.Errorf("index.json does not list %s", name)
		}
		checkProtocolV2Transcript(t, filepath.Join(dir, name))
	}
	if transcripts != len(index.Transcripts) {
		t.Fatalf("%d transcript files, index.json has %d", transcripts, len(index.Transcripts))
	}

	var vectors struct {
		Schema string `json:"schema"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "test_vectors.json")); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(b, &vectors); err != nil {
		t.Fatalf("test_vectors.json: %v", err)
	}
	if vectors.Schema != "spellbench-test-vectors/v2" {
		t.Fatalf("test_vectors.json schema %q", vectors.Schema)
	}
}

func checkProtocolV2Transcript(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 9<<20)
	lines := 0
	for sc.Scan() {
		lines++
		var record struct {
			Dir     string          `json:"dir"`
			Message json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(sc.Bytes(), &record); err != nil {
			t.Fatalf("%s:%d: %v", path, lines, err)
		}
		switch record.Dir {
		case "host_to_engine", "engine_to_host", "host_to_agent", "agent_to_host":
		default:
			t.Fatalf("%s:%d: dir %q", path, lines, record.Dir)
		}
		if len(record.Message) == 0 || !json.Valid(record.Message) {
			t.Fatalf("%s:%d: invalid message", path, lines)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if lines == 0 {
		t.Fatalf("%s: empty transcript", path)
	}
}
