package review

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// A review is one room: every member of one or several councils, one
// prompt, answered in one pass by whoever has the model (the user's AI
// tool, or a model API). Council builds the prompt, then checks the answer
// and renders the outcome.

//go:embed room.md
var roomText string

var roomTemplate = template.Must(template.New("room").Parse(roomText))

// debateFields is the JSON shape of one council's debate.
const debateFields = `"reviews":[{"expert":"<member id>","verdict":"<pass|comment|block|escalate>","confidence":<0.0-1.0>,"notes":["<observation>"],"replies":[{"to":"<an earlier member's id>","stance":"<agree|disagree|adds>","note":"<their reaction>"}]}],` +
	`"final_words":[{"expert":"<member id>","verdict":"<their verdict now>","change_reason":"<what convinced them, or empty>","replies":[{"to":"<a later member's id>","stance":"<agree|disagree|adds>","note":"<their answer>"}]}],` +
	`"disagreements":[{"topic":"<the open question>","sides":[{"experts":["<member id>"],"position":"<what they hold>"}]}],` +
	`"decisions":["<a question the human must answer, naming what each choice costs>"],` +
	`"agreements":["<a point nobody disputed>"]`

// BuildRoomPrompt builds the prompt for a review by one council, or by
// several that then answer each other.
func BuildRoomPrompt(councils []Council, sub Submission) string {
	names := make([]string, len(councils))
	for i, c := range councils {
		names[i] = c.Name
	}
	var buf bytes.Buffer
	err := roomTemplate.Execute(&buf, struct {
		Councils     []Council
		Multi        bool
		Names        string
		Sub          Submission
		DebateFields string
	}{councils, len(councils) > 1, strings.Join(names, ", "), sub, debateFields})
	if err != nil {
		// The template is fixed and tested; this can't happen with valid data.
		panic(fmt.Sprintf("room prompt: %v", err))
	}
	return buf.String()
}

// The answer's JSON, as the room prompt asks for it.
type roomDebate struct {
	Council       string         `json:"council"`
	Reviews       []roomReview   `json:"reviews"`
	FinalWords    []roomReview   `json:"final_words"`
	Disagreements []Disagreement `json:"disagreements"`
	Decisions     []string       `json:"decisions"`
	Agreements    []string       `json:"agreements"`
}

type roomReview struct {
	Expert       string   `json:"expert"`
	Verdict      Verdict  `json:"verdict"`
	Confidence   float64  `json:"confidence"`
	Notes        []string `json:"notes"`
	Replies      []Reply  `json:"replies"`
	ChangeReason string   `json:"change_reason"`
}

type roomAnswer struct {
	roomDebate
	Councils   []roomDebate       `json:"councils"`
	Statements []CouncilStatement `json:"statements"`
}

// AnswerError lists what is wrong with an answer to a room prompt, in
// words the AI can act on.
type AnswerError struct {
	Problems []string
}

func (e *AnswerError) Error() string {
	return "the answer needs fixing; answer again with the whole JSON object:\n- " + strings.Join(e.Problems, "\n- ")
}

// ParseRoom checks the answer to the room prompt for these councils and
// returns the outcome. For one council the outcome holds that council
// alone. On a bad answer it returns an *AnswerError.
func ParseRoom(answer string, councils []Council) (*CouncilsResult, error) {
	var a roomAnswer
	if err := decodeAnswer(answer, &a); err != nil {
		return nil, &AnswerError{Problems: []string{err.Error()}}
	}

	var problems []string
	out := &CouncilsResult{}

	if len(councils) == 1 {
		result, p := checkDebate(a.roomDebate, councils[0], "")
		out.Councils = []CouncilOutcome{{Name: councils[0].Name, Result: result}}
		problems = append(problems, p...)
	} else {
		byName := map[string]roomDebate{}
		for _, d := range a.Councils {
			if _, dup := byName[d.Council]; dup {
				problems = append(problems, fmt.Sprintf("councils: %q appears twice", d.Council))
			}
			byName[d.Council] = d
		}
		names := map[string]bool{}
		for _, c := range councils {
			names[c.Name] = true
			d, ok := byName[c.Name]
			if !ok {
				problems = append(problems, fmt.Sprintf("councils: no debate for the %q council", c.Name))
				continue
			}
			result, p := checkDebate(d, c, c.Name+" council: ")
			out.Councils = append(out.Councils, CouncilOutcome{Name: c.Name, Result: result})
			problems = append(problems, p...)
		}
		for name := range byName {
			if !names[name] {
				problems = append(problems, fmt.Sprintf("councils: %q is not one of the councils (%s)", name, councilNames(councils)))
			}
		}

		spoke := map[string]bool{}
		for _, s := range a.Statements {
			switch {
			case !names[s.Council]:
				problems = append(problems, fmt.Sprintf("statements: %q is not one of the councils (%s)", s.Council, councilNames(councils)))
				continue
			case spoke[s.Council]:
				problems = append(problems, fmt.Sprintf("statements: the %s council speaks twice", s.Council))
				continue
			case strings.TrimSpace(s.Position) == "":
				problems = append(problems, fmt.Sprintf("statements: the %s council's position is empty", s.Council))
			}
			spoke[s.Council] = true
			problems = append(problems, checkReplies(s.Challenges, names, s.Council, "statements: the "+s.Council+" council's challenges")...)
			s.Position = strings.TrimSpace(s.Position)
			out.Statements = append(out.Statements, s)
		}
		for _, c := range councils {
			if !spoke[c.Name] {
				problems = append(problems, fmt.Sprintf("statements: no statement from the %s council's spokesperson", c.Name))
			}
		}

		problems = append(problems, checkDisagreements(a.Disagreements, names, "council name", "")...)
		out.Disagreements = a.Disagreements
		out.Decisions = nonEmpty(a.Decisions)
		out.Agreements = nonEmpty(a.Agreements)
	}

	if len(problems) > 0 {
		return nil, &AnswerError{Problems: problems}
	}
	return out, nil
}

// checkDebate checks one council's debate and turns it into a result.
func checkDebate(d roomDebate, c Council, prefix string) (*SynthesizedResult, []string) {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, prefix+fmt.Sprintf(format, args...)) }

	ids := map[string]bool{}
	for _, in := range c.Inputs {
		ids[in.Expert.ID] = true
	}

	reviewed := map[string]*ExpertVerdict{}
	var order []string
	for _, r := range d.Reviews {
		switch {
		case !ids[r.Expert]:
			add("reviews: %q is not a member (use one of: %s)", r.Expert, memberIDs(c))
			continue
		case reviewed[r.Expert] != nil:
			add("reviews: %s reviews twice; each member reviews once", r.Expert)
			continue
		case !ValidVerdicts[r.Verdict]:
			add("reviews: %s has verdict %q; use pass, comment, block, or escalate", r.Expert, r.Verdict)
		}
		notes := nonEmpty(r.Notes)
		if len(notes) == 0 {
			add("reviews: %s has no notes", r.Expert)
		}
		problems = append(problems, checkReplies(r.Replies, ids, r.Expert, prefix+"reviews: "+r.Expert+"'s replies")...)
		reviewed[r.Expert] = &ExpertVerdict{
			Expert: r.Expert, Verdict: r.Verdict, Confidence: clamp01(r.Confidence),
			Notes: notes, Replies: normalizeReplies(r.Replies),
		}
		order = append(order, r.Expert)
	}
	for _, in := range c.Inputs {
		if reviewed[in.Expert.ID] == nil {
			add("reviews: no review from %s (%s)", in.Expert.ID, in.Expert.Name)
		}
	}

	for _, f := range d.FinalWords {
		own := reviewed[f.Expert]
		if own == nil {
			add("final_words: %q has no review; only members who reviewed get a final word", f.Expert)
			continue
		}
		if f.Verdict != "" && !ValidVerdicts[f.Verdict] {
			add("final_words: %s has verdict %q; use pass, comment, block, or escalate", f.Expert, f.Verdict)
			continue
		}
		problems = append(problems, checkReplies(f.Replies, ids, f.Expert, prefix+"final_words: "+f.Expert+"'s replies")...)
		own.FinalWord = normalizeReplies(f.Replies)
		if f.Verdict != "" && f.Verdict != own.Verdict {
			own.ChangedFrom = own.Verdict
			own.Verdict = f.Verdict
			own.ChangeReason = strings.TrimSpace(f.ChangeReason)
		}
	}

	problems = append(problems, checkDisagreements(d.Disagreements, ids, "member id", prefix)...)

	if len(problems) > 0 {
		return nil, problems
	}

	byID := map[string]ExpertInput{}
	for _, in := range c.Inputs {
		byID[in.Expert.ID] = in
	}
	verdicts := make([]ExpertVerdict, 0, len(order))
	for _, id := range order {
		v := *reviewed[id]
		v.Name = byID[id].Expert.Name
		v.Blocking = byID[id].Blocking
		verdicts = append(verdicts, v)
	}
	result := Synthesize(verdicts, c.Experts(), nil)
	result.Disagreements = d.Disagreements
	result.Decisions = nonEmpty(d.Decisions)
	if a := nonEmpty(d.Agreements); len(a) > 0 {
		result.Agreements = a
	}
	return result, nil
}

// checkReplies checks that replies go to someone else in the room, with a
// known stance and a note.
func checkReplies(replies []Reply, valid map[string]bool, self, where string) []string {
	var problems []string
	for _, r := range replies {
		switch {
		case r.To == self:
			problems = append(problems, fmt.Sprintf("%s: a reply to themselves", where))
		case !valid[r.To]:
			problems = append(problems, fmt.Sprintf("%s: %q is not in the room", where, r.To))
		}
		switch Stance(strings.ToLower(strings.TrimSpace(string(r.Stance)))) {
		case StanceAgree, StanceDisagree, StanceAdds:
		default:
			problems = append(problems, fmt.Sprintf("%s: stance %q; use agree, disagree, or adds", where, r.Stance))
		}
		if strings.TrimSpace(r.Note) == "" {
			problems = append(problems, fmt.Sprintf("%s: a reply to %s has no note", where, r.To))
		}
	}
	return problems
}

// checkDisagreements checks that each disagreement has a topic and at
// least two sides held by parties in the room.
func checkDisagreements(ds []Disagreement, valid map[string]bool, what, prefix string) []string {
	var problems []string
	for i, d := range ds {
		where := fmt.Sprintf("%sdisagreements[%d]", prefix, i)
		if strings.TrimSpace(d.Topic) == "" {
			problems = append(problems, where+": no topic")
		}
		if len(d.Sides) < 2 {
			problems = append(problems, where+": needs at least two sides; leave out points nobody disputes")
		}
		for _, s := range d.Sides {
			if strings.TrimSpace(s.Position) == "" {
				problems = append(problems, where+": a side with no position")
			}
			if len(s.Experts) == 0 {
				problems = append(problems, where+": a side with nobody holding it")
			}
			for _, id := range s.Experts {
				if !valid[id] {
					problems = append(problems, fmt.Sprintf("%s: %q is not a %s in the room", where, id, what))
				}
			}
		}
	}
	return problems
}

// decodeAnswer reads the JSON object in an answer: the whole text, a code
// fence, or the outermost braces.
func decodeAnswer(answer string, v any) error {
	text := strings.TrimSpace(answer)
	if text == "" {
		return fmt.Errorf("the answer is empty; it should be the JSON object the room prompt asks for")
	}
	var firstErr error
	candidates := []string{text, extractFromCodeFence(text)}
	if start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}'); start >= 0 && end > start {
		candidates = append(candidates, text[start:end+1])
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		err := json.Unmarshal([]byte(c), v)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return fmt.Errorf("the answer is not valid JSON: %v", firstErr)
}

func memberIDs(c Council) string {
	ids := make([]string, len(c.Inputs))
	for i, in := range c.Inputs {
		ids[i] = in.Expert.ID
	}
	return strings.Join(ids, ", ")
}

func councilNames(councils []Council) string {
	names := make([]string, len(councils))
	for i, c := range councils {
		names[i] = c.Name
	}
	return strings.Join(names, ", ")
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// FormatRoom renders a recorded review for people.
func FormatRoom(r *CouncilsResult) string {
	if len(r.Councils) == 1 {
		c := r.Councils[0]
		return FormatHuman(c.Result, c.Name, len(c.Result.Perspectives))
	}
	return FormatHumanCouncils(r)
}
