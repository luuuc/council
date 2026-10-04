package review

import "strings"

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
		r.Note = strings.TrimSpace(r.Note)
		out = append(out, r)
	}
	return out
}

func nonEmpty(items []string) []string {
	var out []string
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
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

// truncateBytes returns a string of at most maxLen bytes from b.
func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
