package review

import (
	"encoding/json"
	"testing"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name       string
		expertID   string
		raw        string
		wantVerdict Verdict
		wantConf   float64
		wantNotes  int
		wantError  bool // expect fallback with Error set
	}{
		{
			name:        "valid JSON",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"comment","confidence":0.85,"notes":["Missing test"],"blocking":false}`,
			wantVerdict: VerdictComment,
			wantConf:    0.85,
			wantNotes:   1,
		},
		{
			name:        "pass verdict",
			expertID:    "bruce-schneier",
			raw:         `{"expert":"bruce-schneier","verdict":"pass","confidence":0.95,"notes":[],"blocking":false}`,
			wantVerdict: VerdictPass,
			wantConf:    0.95,
			wantNotes:   0,
		},
		{
			name:        "block verdict",
			expertID:    "bruce-schneier",
			raw:         `{"expert":"bruce-schneier","verdict":"block","confidence":0.9,"notes":["SQL injection risk"],"blocking":true}`,
			wantVerdict: VerdictBlock,
			wantConf:    0.9,
			wantNotes:   1,
		},
		{
			name:        "escalate verdict",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"escalate","confidence":0.5,"notes":["Needs architect input"],"blocking":false}`,
			wantVerdict: VerdictEscalate,
			wantConf:    0.5,
			wantNotes:   1,
		},
		{
			name:     "JSON in code fence with json tag",
			expertID: "kent-beck",
			raw: "Here's my review:\n```json\n" +
				`{"expert":"kent-beck","verdict":"comment","confidence":0.7,"notes":["Extract method"],"blocking":false}` +
				"\n```\nHope this helps!",
			wantVerdict: VerdictComment,
			wantConf:    0.7,
			wantNotes:   1,
		},
		{
			name:     "JSON in plain code fence",
			expertID: "kent-beck",
			raw: "```\n" +
				`{"expert":"kent-beck","verdict":"pass","confidence":0.9,"notes":[],"blocking":false}` +
				"\n```",
			wantVerdict: VerdictPass,
			wantConf:    0.9,
			wantNotes:   0,
		},
		{
			name:     "JSON embedded in prose",
			expertID: "kent-beck",
			raw: `After careful review, here is my assessment:
{"expert":"kent-beck","verdict":"comment","confidence":0.8,"notes":["Add test"],"blocking":false}
That's my take.`,
			wantVerdict: VerdictComment,
			wantConf:    0.8,
			wantNotes:   1,
		},
		{
			name:        "empty response",
			expertID:    "kent-beck",
			raw:         "",
			wantVerdict: VerdictComment,
			wantConf:    0,
			wantNotes:   1, // fallback note with error snippet
			wantError:   true,
		},
		{
			name:        "whitespace only",
			expertID:    "kent-beck",
			raw:         "   \n  \t  ",
			wantVerdict: VerdictComment,
			wantConf:    0,
			wantNotes:   1,
			wantError:   true,
		},
		{
			name:        "completely unstructured response",
			expertID:    "kent-beck",
			raw:         "I think the code looks pretty good overall. Nice job!",
			wantVerdict: VerdictComment,
			wantConf:    0,
			wantNotes:   1,
			wantError:   true,
		},
		{
			name:        "truncated JSON",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":`,
			wantVerdict: VerdictComment,
			wantConf:    0,
			wantNotes:   1,
			wantError:   true,
		},
		{
			name:        "invalid verdict value",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"maybe","confidence":0.5,"notes":[],"blocking":false}`,
			wantVerdict: VerdictComment,
			wantConf:    0,
			wantNotes:   1,
			wantError:   true,
		},
		{
			name:        "confidence clamped high",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"pass","confidence":5.0,"notes":[],"blocking":false}`,
			wantVerdict: VerdictPass,
			wantConf:    1.0,
			wantNotes:   0,
		},
		{
			name:        "confidence clamped low",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"pass","confidence":-1.0,"notes":[],"blocking":false}`,
			wantVerdict: VerdictPass,
			wantConf:    0,
			wantNotes:   0,
		},
		{
			name:        "notes as string instead of array",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"comment","confidence":0.7,"notes":"single note","blocking":false}`,
			wantVerdict: VerdictComment,
			wantConf:    0.7,
			wantNotes:   1,
		},
		{
			name:        "missing notes field",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"pass","confidence":0.9,"blocking":false}`,
			wantVerdict: VerdictPass,
			wantConf:    0.9,
			wantNotes:   0,
		},
		{
			name:        "extra fields ignored",
			expertID:    "kent-beck",
			raw:         `{"expert":"kent-beck","verdict":"pass","confidence":0.9,"notes":[],"blocking":false,"extra":"ignored"}`,
			wantVerdict: VerdictPass,
			wantConf:    0.9,
			wantNotes:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := ParseVerdict(tt.expertID, []byte(tt.raw))

			if v.Expert != tt.expertID {
				t.Errorf("Expert = %q, want %q", v.Expert, tt.expertID)
			}
			if v.Verdict != tt.wantVerdict {
				t.Errorf("Verdict = %q, want %q", v.Verdict, tt.wantVerdict)
			}
			if v.Confidence != tt.wantConf {
				t.Errorf("Confidence = %v, want %v", v.Confidence, tt.wantConf)
			}
			if len(v.Notes) != tt.wantNotes {
				t.Errorf("Notes count = %d, want %d (notes: %v)", len(v.Notes), tt.wantNotes, v.Notes)
			}
			if tt.wantError && v.Error == "" {
				t.Error("expected Error to be set, got empty")
			}
			if !tt.wantError && v.Error != "" {
				t.Errorf("unexpected Error: %q", v.Error)
			}
		})
	}
}

func TestParseCollectiveResult(t *testing.T) {
	expected := []string{"kent-beck", "bruce-schneier"}

	tests := []struct {
		name             string
		raw              string
		expected         []string
		wantVerdict      Verdict
		wantPerspectives int
		wantAgreements   int
		wantTension      bool
		wantFallback     bool
	}{
		{
			name: "valid collective JSON",
			raw: mustJSON(map[string]any{
				"verdict":  "comment",
				"blocking": false,
				"perspectives": []map[string]any{
					{"expert": "kent-beck", "verdict": "comment", "confidence": 0.8, "notes": []string{"Add test"}, "blocking": false},
					{"expert": "bruce-schneier", "verdict": "pass", "confidence": 0.95, "notes": []string{}, "blocking": false},
				},
				"agreements": []string{"Code structure is clean"},
				"tension":    "Kent wants tests, Bruce satisfied",
				"summary":    "Ship with comments.",
			}),
			expected:         expected,
			wantVerdict:      VerdictComment,
			wantPerspectives: 2,
			wantAgreements:   1,
			wantTension:      true,
		},
		{
			name: "JSON in code fences",
			raw: "Here's my review:\n```json\n" + mustJSON(map[string]any{
				"verdict":  "pass",
				"blocking": false,
				"perspectives": []map[string]any{
					{"expert": "kent-beck", "verdict": "pass", "confidence": 0.9, "notes": []string{}, "blocking": false},
					{"expert": "bruce-schneier", "verdict": "pass", "confidence": 0.95, "notes": []string{}, "blocking": false},
				},
				"agreements": []string{},
				"tension":    "",
				"summary":    "Ship it.",
			}) + "\n```\nDone.",
			expected:         expected,
			wantVerdict:      VerdictPass,
			wantPerspectives: 2,
		},
		{
			name: "missing perspective filled in",
			raw: mustJSON(map[string]any{
				"verdict":  "comment",
				"blocking": false,
				"perspectives": []map[string]any{
					{"expert": "kent-beck", "verdict": "comment", "confidence": 0.8, "notes": []string{"Add test"}, "blocking": false},
				},
				"agreements": []string{},
				"tension":    "",
				"summary":    "Review.",
			}),
			expected:         expected,
			wantVerdict:      VerdictComment,
			wantPerspectives: 2,
		},
		{
			name: "unknown expert dropped",
			raw: mustJSON(map[string]any{
				"verdict":  "pass",
				"blocking": false,
				"perspectives": []map[string]any{
					{"expert": "kent-beck", "verdict": "pass", "confidence": 0.9, "notes": []string{}, "blocking": false},
					{"expert": "bruce-schneier", "verdict": "pass", "confidence": 0.95, "notes": []string{}, "blocking": false},
					{"expert": "invented-expert", "verdict": "block", "confidence": 0.5, "notes": []string{"Invented"}, "blocking": false},
				},
				"agreements": []string{},
				"tension":    "",
				"summary":    "Ship it.",
			}),
			expected:         expected,
			wantVerdict:      VerdictPass,
			wantPerspectives: 2,
		},
		{
			name: "invalid verdict normalized",
			raw: mustJSON(map[string]any{
				"verdict":  "maybe",
				"blocking": false,
				"perspectives": []map[string]any{
					{"expert": "kent-beck", "verdict": "maybe", "confidence": 0.8, "notes": []string{"Unsure"}, "blocking": false},
					{"expert": "bruce-schneier", "verdict": "pass", "confidence": 0.9, "notes": []string{}, "blocking": false},
				},
				"agreements": []string{},
				"tension":    "",
				"summary":    "Unclear.",
			}),
			expected:         expected,
			wantVerdict:      VerdictComment,
			wantPerspectives: 2,
		},
		{
			name:             "empty response",
			raw:              "",
			expected:         expected,
			wantVerdict:      VerdictComment,
			wantPerspectives: 2,
			wantFallback:     true,
		},
		{
			name:             "completely unstructured",
			raw:              "I think the code looks pretty good overall!",
			expected:         expected,
			wantVerdict:      VerdictComment,
			wantPerspectives: 2,
			wantFallback:     true,
		},
		{
			name:             "truncated JSON",
			raw:              `{"verdict":"pass","perspectives":[{"expert":"kent-beck"`,
			expected:         expected,
			wantVerdict:      VerdictComment,
			wantPerspectives: 2,
			wantFallback:     true,
		},
		{
			name: "confidence clamped",
			raw: mustJSON(map[string]any{
				"verdict":  "pass",
				"blocking": false,
				"perspectives": []map[string]any{
					{"expert": "kent-beck", "verdict": "pass", "confidence": 5.0, "notes": []string{}, "blocking": false},
					{"expert": "bruce-schneier", "verdict": "pass", "confidence": -1.0, "notes": []string{}, "blocking": false},
				},
				"agreements": []string{},
				"tension":    "",
				"summary":    "Ship.",
			}),
			expected:         expected,
			wantVerdict:      VerdictPass,
			wantPerspectives: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCollectiveResult([]byte(tt.raw), tt.expected)

			if result.Verdict != tt.wantVerdict {
				t.Errorf("Verdict = %q, want %q", result.Verdict, tt.wantVerdict)
			}
			if len(result.Perspectives) != tt.wantPerspectives {
				t.Errorf("Perspectives = %d, want %d", len(result.Perspectives), tt.wantPerspectives)
			}
			if tt.wantAgreements > 0 && len(result.Agreements) != tt.wantAgreements {
				t.Errorf("Agreements = %d, want %d", len(result.Agreements), tt.wantAgreements)
			}
			if tt.wantTension && result.Tension == "" {
				t.Error("expected tension to be set")
			}
			if tt.wantFallback {
				for _, p := range result.Perspectives {
					if p.Error == "" {
						t.Errorf("expected fallback Error for %s", p.Expert)
					}
				}
			}
		})
	}
}

func TestParseCollectiveResultNeverPanics(t *testing.T) {
	inputs := []string{
		"",
		"   ",
		"{}",
		"[]",
		`{"perspectives": "not an array"}`,
		`{"perspectives": []}`,
		"null",
		`{"verdict": "pass"}`,
	}

	for _, input := range inputs {
		result := ParseCollectiveResult([]byte(input), []string{"expert-a"})
		if result == nil {
			t.Errorf("ParseCollectiveResult returned nil for input %q", input)
		}
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestParseVerdictReplies(t *testing.T) {
	raw := `Here is my review:
{"expert":"kent-beck","verdict":"comment","confidence":0.8,"notes":["Add a test"],
 "replies":[
  {"to":"dhh","stance":"Disagree","note":"The layers make it testable"},
  {"to":"rob-pike","stance":"shrug","note":"unknown stance is dropped"},
  {"to":"","stance":"agree","note":"missing target is dropped"},
  {"to":"rob-pike","stance":"adds","note":""}
 ],"blocking":false}`

	v := ParseVerdict("kent-beck", []byte(raw))

	if v.Error != "" {
		t.Fatalf("unexpected parse error: %s (notes: %v)", v.Error, v.Notes)
	}
	if len(v.Replies) != 1 {
		t.Fatalf("expected 1 valid reply, got %d: %+v", len(v.Replies), v.Replies)
	}
	want := Reply{To: "dhh", Stance: StanceDisagree, Note: "The layers make it testable"}
	if v.Replies[0] != want {
		t.Errorf("reply = %+v, want %+v", v.Replies[0], want)
	}
}
