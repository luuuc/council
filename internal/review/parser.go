package review

import (
	"encoding/json"
	"regexp"
	"strings"
)

// jsonObjectRe matches a JSON object containing a "verdict" key.
var jsonObjectRe = regexp.MustCompile(`\{[^{}]*"verdict"[^{}]*\}`)

// ParseVerdict extracts a structured ExpertVerdict from raw LLM output.
// It tries multiple extraction strategies in order of specificity,
// falling back to a low-confidence comment verdict if nothing works.
func ParseVerdict(expertID string, raw []byte) ExpertVerdict {
	text := strings.TrimSpace(string(raw))

	if text == "" {
		return fallbackVerdict(expertID, "empty response")
	}

	// Strategy 1: direct JSON unmarshal
	if v, ok := tryUnmarshal(expertID, []byte(text)); ok {
		return v
	}

	// Strategy 2: extract from code fences (```json ... ``` or ``` ... ```)
	if extracted := extractFromCodeFence(text); extracted != "" {
		if v, ok := tryUnmarshal(expertID, []byte(extracted)); ok {
			return v
		}
	}

	// Strategy 3: outermost braces (handles nested objects like replies)
	if start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}'); start >= 0 && end > start {
		if v, ok := tryUnmarshal(expertID, []byte(text[start:end+1])); ok {
			return v
		}
	}

	// Strategy 4: regex extract flat JSON object containing "verdict"
	if match := jsonObjectRe.FindString(text); match != "" {
		if v, ok := tryUnmarshal(expertID, []byte(match)); ok {
			return v
		}
	}

	// Strategy 5: fallback
	return fallbackVerdict(expertID, truncate(text, 200))
}

// tryUnmarshal attempts to parse JSON into an ExpertVerdict, validates, and normalizes.
func tryUnmarshal(expertID string, data []byte) (ExpertVerdict, bool) {
	var raw struct {
		Expert       string      `json:"expert"`
		Verdict      Verdict     `json:"verdict"`
		Confidence   float64     `json:"confidence"`
		Notes        interface{} `json:"notes"`
		Replies      []Reply     `json:"replies"`
		Blocking     bool        `json:"blocking"`
		ChangeReason string      `json:"change_reason"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return ExpertVerdict{}, false
	}

	// Validate verdict
	if !ValidVerdicts[raw.Verdict] {
		return ExpertVerdict{}, false
	}

	// Normalize confidence to [0, 1]
	if raw.Confidence < 0 {
		raw.Confidence = 0
	}
	if raw.Confidence > 1 {
		raw.Confidence = 1
	}

	// Normalize notes: accept string or []string
	notes := normalizeNotes(raw.Notes)

	return ExpertVerdict{
		Expert:       expertID,
		Verdict:      raw.Verdict,
		Confidence:   raw.Confidence,
		Notes:        notes,
		Replies:      normalizeReplies(raw.Replies),
		Blocking:     raw.Blocking,
		ChangeReason: strings.TrimSpace(raw.ChangeReason),
	}, true
}

// normalizeReplies drops replies with no target, no note, or an unknown stance.
func normalizeReplies(replies []Reply) []Reply {
	var out []Reply
	for _, r := range replies {
		r.Stance = Stance(strings.ToLower(strings.TrimSpace(string(r.Stance))))
		switch r.Stance {
		case StanceAgree, StanceDisagree, StanceAdds:
		default:
			continue
		}
		if r.To == "" || strings.TrimSpace(r.Note) == "" {
			continue
		}
		out = append(out, r)
	}
	return out
}

// normalizeNotes converts various note formats to []string.
func normalizeNotes(v interface{}) []string {
	switch n := v.(type) {
	case []interface{}:
		notes := make([]string, 0, len(n))
		for _, item := range n {
			if s, ok := item.(string); ok && s != "" {
				notes = append(notes, s)
			}
		}
		return notes
	case string:
		if n != "" {
			return []string{n}
		}
		return nil
	default:
		return nil
	}
}

// extractFromCodeFence extracts content from markdown code fences.
func extractFromCodeFence(text string) string {
	// Try ```json first
	if idx := strings.Index(text, "```json"); idx >= 0 {
		content := text[idx+7:]
		if end := strings.Index(content, "```"); end >= 0 {
			return strings.TrimSpace(content[:end])
		}
	}

	// Try plain ```
	if idx := strings.Index(text, "```"); idx >= 0 {
		content := text[idx+3:]
		// Skip optional language tag on same line
		if nl := strings.IndexByte(content, '\n'); nl >= 0 {
			content = content[nl+1:]
		}
		if end := strings.Index(content, "```"); end >= 0 {
			return strings.TrimSpace(content[:end])
		}
	}

	return ""
}

// fallbackVerdict creates a low-confidence comment verdict when parsing fails.
func fallbackVerdict(expertID, rawSnippet string) ExpertVerdict {
	return ExpertVerdict{
		Expert:     expertID,
		Verdict:    VerdictComment,
		Confidence: 0,
		Notes:      []string{"Response could not be parsed: " + rawSnippet},
		Blocking:   false,
		Error:      "unparseable response",
	}
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// collectiveJSONStartRe matches the opening of a collective JSON object containing a "perspectives" array.
var collectiveJSONStartRe = regexp.MustCompile(`\{[^{}]*"perspectives"\s*:\s*\[`)

// ParseCollectiveResult extracts a SynthesizedResult from raw LLM output.
// expectedExperts is the list of expert IDs that should be in the response.
func ParseCollectiveResult(raw []byte, expectedExperts []string) *SynthesizedResult {
	text := strings.TrimSpace(string(raw))

	if text == "" {
		return collectiveFallback("empty response", expectedExperts)
	}

	// Strategy 1: direct JSON unmarshal
	if r, ok := tryUnmarshalCollective(text, expectedExperts); ok {
		return r
	}

	// Strategy 2: extract from code fences
	if extracted := extractFromCodeFence(text); extracted != "" {
		if r, ok := tryUnmarshalCollective(extracted, expectedExperts); ok {
			return r
		}
	}

	// Strategy 3: find JSON object containing "perspectives" array
	if loc := collectiveJSONStartRe.FindStringIndex(text); loc != nil {
		candidate := extractBalancedJSON(text[loc[0]:])
		if candidate != "" {
			if r, ok := tryUnmarshalCollective(candidate, expectedExperts); ok {
				return r
			}
		}
	}

	// Strategy 4: fallback
	return collectiveFallback(truncate(text, 200), expectedExperts)
}

// tryUnmarshalCollective attempts to parse JSON into a SynthesizedResult.
func tryUnmarshalCollective(text string, expectedExperts []string) (*SynthesizedResult, bool) {
	var raw struct {
		Verdict      Verdict `json:"verdict"`
		Blocking     bool    `json:"blocking"`
		Perspectives []struct {
			Expert     string      `json:"expert"`
			Verdict    Verdict     `json:"verdict"`
			Confidence float64     `json:"confidence"`
			Notes      interface{} `json:"notes"`
			Blocking   bool        `json:"blocking"`
		} `json:"perspectives"`
		Agreements    []string       `json:"agreements"`
		Disagreements []Disagreement `json:"disagreements"`
		Decisions     []string       `json:"decisions"`
		Tension       string         `json:"tension"`
		Summary       string         `json:"summary"`
	}

	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, false
	}

	if len(raw.Perspectives) == 0 {
		return nil, false
	}

	expected := make(map[string]bool, len(expectedExperts))
	for _, id := range expectedExperts {
		expected[id] = true
	}

	var perspectives []ExpertVerdict
	seen := make(map[string]bool)
	for _, p := range raw.Perspectives {
		if !expected[p.Expert] {
			continue
		}
		if seen[p.Expert] {
			continue
		}
		seen[p.Expert] = true

		verdict := p.Verdict
		if !ValidVerdicts[verdict] {
			verdict = VerdictComment
		}

		conf := p.Confidence
		if conf < 0 {
			conf = 0
		}
		if conf > 1 {
			conf = 1
		}

		perspectives = append(perspectives, ExpertVerdict{
			Expert:     p.Expert,
			Verdict:    verdict,
			Confidence: conf,
			Notes:      normalizeNotes(p.Notes),
			Blocking:   p.Blocking,
		})
	}

	// Fill in missing experts
	for _, id := range expectedExperts {
		if !seen[id] {
			perspectives = append(perspectives, ExpertVerdict{
				Expert:     id,
				Verdict:    VerdictComment,
				Confidence: 0,
				Notes:      []string{"No perspective provided by model"},
				Error:      "missing from collective response",
			})
		}
	}

	overall := raw.Verdict
	if !ValidVerdicts[overall] {
		overall = VerdictComment
	}

	// The summary is a neutral vote count; a model's recommendation is dropped.
	return &SynthesizedResult{
		Verdict:       overall,
		Blocking:      raw.Blocking,
		Perspectives:  perspectives,
		Agreements:    raw.Agreements,
		Disagreements: cleanDisagreements(raw.Disagreements, expected),
		Decisions:     nonEmpty(raw.Decisions),
		Tension:       raw.Tension,
		Summary:       buildSummary(perspectives, nil, overall, raw.Blocking),
	}, true
}

// extractBalancedJSON extracts a complete JSON object from text starting at '{'.
func extractBalancedJSON(text string) string {
	if len(text) == 0 || text[0] != '{' {
		return ""
	}

	depth := 0
	inString := false
	escape := false

	for i, ch := range text {
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[:i+1]
			}
		}
	}
	return ""
}

// collectiveFallback creates a minimal SynthesizedResult when parsing fails.
func collectiveFallback(rawSnippet string, expectedExperts []string) *SynthesizedResult {
	perspectives := make([]ExpertVerdict, len(expectedExperts))
	for i, id := range expectedExperts {
		perspectives[i] = ExpertVerdict{
			Expert:     id,
			Verdict:    VerdictComment,
			Confidence: 0,
			Notes:      []string{"Response could not be parsed: " + rawSnippet},
			Error:      "unparseable collective response",
		}
	}

	return &SynthesizedResult{
		Verdict:      VerdictComment,
		Perspectives: perspectives,
		Summary:      "Collective response could not be parsed.",
	}
}
