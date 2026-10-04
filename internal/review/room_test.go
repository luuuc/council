package review

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func roomCouncil(name string, ids ...string) Council {
	c := Council{Name: name}
	for _, id := range ids {
		c.Inputs = append(c.Inputs, ExpertInput{Expert: &expert.Expert{
			ID: id, Name: "Virtual " + strings.ToUpper(id[:1]) + id[1:], Kind: expert.KindPerson,
			Focus: "testing", Body: "Persona of " + id + ".",
		}})
	}
	return c
}

func TestBuildRoomPromptOneCouncil(t *testing.T) {
	c := roomCouncil("code", "ada", "ben")
	c.Inputs[1].Blocking = true
	c.Inputs[1].Expert.Kind = expert.KindCustomer
	prompt := BuildRoomPrompt([]Council{c}, Submission{Content: "the diff", Context: "File: main.go"})

	for _, want := range []string{
		"in one pass",
		"Final word", "Moderator", "No recommendation",
		"### Virtual Ada (id: ada, kind: person)",
		"### Virtual Ben (id: ben, kind: customer, blocking member)",
		"Persona of ada.",
		"the diff", "File: main.go",
		`"reviews":[`, `"final_words":[`, `"disagreements":[`,
		"path/to/file.go:42:",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
	for _, unwanted := range []string{"Spokespersons", `"councils":[`, "<no value>"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("one-council prompt should not contain %q", unwanted)
		}
	}
	if strings.Index(prompt, "Virtual Ada") > strings.Index(prompt, "Virtual Ben") {
		t.Error("members should be listed in pack order")
	}
}

func TestBuildRoomPromptSeveralCouncils(t *testing.T) {
	prompt := BuildRoomPrompt([]Council{roomCouncil("product", "ada"), roomCouncil("security", "ben")}, Submission{Content: "plan"})
	for _, want := range []string{
		"## The product council", "## The security council",
		"Spokespersons", "Moderator across councils",
		`"councils":[{"council":"<council name>","reviews":[`, `"statements":[`,
		"council names (product, security)",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}

const goodDebate = `{
  "reviews": [
    {"expert":"ada","verdict":"block","confidence":0.8,"notes":["main.go:3: no error handling"]},
    {"expert":"ben","verdict":"pass","confidence":1.4,"notes":["ship it"],"replies":[{"to":"ada","stance":"disagree","note":"it can't fail here"}]}
  ],
  "final_words": [
    {"expert":"ada","verdict":"comment","change_reason":"Ben showed the write can't fail","replies":[{"to":"ben","stance":"agree","note":"fair"}]}
  ],
  "disagreements": [{"topic":"error handling","sides":[{"experts":["ada"],"position":"handle it"},{"experts":["ben"],"position":"not needed"}]}],
  "decisions": ["Handle the error now, or later?"],
  "agreements": ["the feature is wanted"]
}`

func TestParseRoomOneCouncil(t *testing.T) {
	c := roomCouncil("code", "ada", "ben")
	c.Inputs[0].Blocking = true

	// Models sometimes wrap JSON in a fence; that's fine.
	out, err := ParseRoom("Here it is:\n```json\n"+goodDebate+"\n```", []Council{c})
	if err != nil {
		t.Fatalf("ParseRoom: %v", err)
	}
	if len(out.Councils) != 1 || out.Councils[0].Name != "code" {
		t.Fatalf("councils = %+v", out.Councils)
	}
	r := out.Councils[0].Result

	ada, ben := r.Perspectives[0], r.Perspectives[1]
	if ada.Name != "Virtual Ada" || !ada.Blocking {
		t.Errorf("ada = %+v, want the name and blocking status from the pack", ada)
	}
	if ada.Verdict != VerdictComment || ada.ChangedFrom != VerdictBlock || ada.ChangeReason == "" || len(ada.FinalWord) != 1 {
		t.Errorf("ada's final word not applied: %+v", ada)
	}
	if ben.Confidence != 1 {
		t.Errorf("confidence = %v, want clamped to 1", ben.Confidence)
	}
	if len(ben.Replies) != 1 || ben.Replies[0].Stance != StanceDisagree {
		t.Errorf("ben's replies = %+v", ben.Replies)
	}
	if r.Verdict != VerdictComment {
		t.Errorf("verdict = %s, want the most severe after final words (comment)", r.Verdict)
	}
	if len(r.Disagreements) != 1 || len(r.Decisions) != 1 || r.Agreements[0] != "the feature is wanted" {
		t.Errorf("moderator not kept: %+v", r)
	}

	text := FormatRoom(out)
	for _, want := range []string{"pack: code", "Virtual Ada", "final word", "Where they disagree", "What you need to decide"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered review is missing %q:\n%s", want, text)
		}
	}
}

func TestParseRoomSeveralCouncils(t *testing.T) {
	councils := []Council{roomCouncil("product", "ada"), roomCouncil("security", "ben")}
	answer := `{
	  "councils": [
	    {"council":"product","reviews":[{"expert":"ada","verdict":"pass","notes":["users want it"]}]},
	    {"council":"security","reviews":[{"expert":"ben","verdict":"block","notes":["leaks tokens"]}]}
	  ],
	  "statements": [
	    {"council":"product","position":"Ship it.","challenges":[{"to":"security","stance":"disagree","note":"risk is small"}]},
	    {"council":"security","position":"Not yet.","challenges":[]}
	  ],
	  "disagreements": [{"topic":"ship now?","sides":[{"experts":["product"],"position":"yes"},{"experts":["security"],"position":"no"}]}],
	  "decisions": ["Ship before the fix?"]
	}`
	out, err := ParseRoom(answer, councils)
	if err != nil {
		t.Fatalf("ParseRoom: %v", err)
	}
	if len(out.Councils) != 2 || len(out.Statements) != 2 || len(out.Disagreements) != 1 {
		t.Fatalf("out = %+v", out)
	}
	text := FormatRoom(out)
	for _, want := range []string{"The product council", "The councils answer each other", "the security council", "Ship before the fix?"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered review is missing %q:\n%s", want, text)
		}
	}
}

func TestParseRoomProblems(t *testing.T) {
	one := []Council{roomCouncil("code", "ada", "ben")}
	two := []Council{roomCouncil("product", "ada"), roomCouncil("security", "ben")}
	review := func(id, verdict string) string {
		return `{"expert":"` + id + `","verdict":"` + verdict + `","notes":["n"]}`
	}

	tests := []struct {
		name     string
		councils []Council
		answer   string
		want     string
	}{
		{"empty", one, "  ", "empty"},
		{"not json", one, "I think it's fine.", "not valid JSON"},
		{"missing member", one, `{"reviews":[` + review("ada", "pass") + `]}`, "no review from ben"},
		{"unknown member", one, `{"reviews":[` + review("ada", "pass") + `,` + review("ben", "pass") + `,` + review("zed", "pass") + `]}`, `"zed" is not a member`},
		{"twice", one, `{"reviews":[` + review("ada", "pass") + `,` + review("ada", "pass") + `,` + review("ben", "pass") + `]}`, "ada reviews twice"},
		{"bad verdict", one, `{"reviews":[` + review("ada", "maybe") + `,` + review("ben", "pass") + `]}`, `verdict "maybe"`},
		{"no notes", one, `{"reviews":[{"expert":"ada","verdict":"pass"},` + review("ben", "pass") + `]}`, "ada has no notes"},
		{"reply to stranger", one, `{"reviews":[` + review("ada", "pass") + `,{"expert":"ben","verdict":"pass","notes":["n"],"replies":[{"to":"zed","stance":"agree","note":"x"}]}]}`, `"zed" is not in the room`},
		{"bad stance", one, `{"reviews":[` + review("ada", "pass") + `,{"expert":"ben","verdict":"pass","notes":["n"],"replies":[{"to":"ada","stance":"meh","note":"x"}]}]}`, `stance "meh"`},
		{"final word without review", one, `{"reviews":[` + review("ada", "pass") + `,` + review("ben", "pass") + `],"final_words":[{"expert":"zed","verdict":"pass"}]}`, `"zed" has no review`},
		{"one-sided disagreement", one, `{"reviews":[` + review("ada", "pass") + `,` + review("ben", "pass") + `],"disagreements":[{"topic":"t","sides":[{"experts":["ada"],"position":"p"}]}]}`, "at least two sides"},
		{"missing council", two, `{"councils":[{"council":"product","reviews":[` + review("ada", "pass") + `]}],"statements":[{"council":"product","position":"p"}]}`, `no debate for the "security" council`},
		{"member in wrong council", two, `{"councils":[{"council":"product","reviews":[` + review("ada", "pass") + `]},{"council":"security","reviews":[` + review("ada", "pass") + `]}],"statements":[{"council":"product","position":"p"},{"council":"security","position":"p"}]}`, "security council: reviews: no review from ben"},
		{"missing statement", two, `{"councils":[{"council":"product","reviews":[` + review("ada", "pass") + `]},{"council":"security","reviews":[` + review("ben", "pass") + `]}],"statements":[{"council":"product","position":"p"}]}`, "no statement from the security council"},
		{"challenge to stranger", two, `{"councils":[{"council":"product","reviews":[` + review("ada", "pass") + `]},{"council":"security","reviews":[` + review("ben", "pass") + `]}],"statements":[{"council":"product","position":"p","challenges":[{"to":"legal","stance":"agree","note":"x"}]},{"council":"security","position":"p"}]}`, `"legal" is not in the room`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRoom(tt.answer, tt.councils)
			var ae *AnswerError
			if !errors.As(err, &ae) {
				t.Fatalf("err = %v, want an *AnswerError", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v\nwant it to mention %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), "answer again") {
				t.Errorf("err should tell the AI to answer again: %v", err)
			}
		})
	}
}

func TestSeatCouncils(t *testing.T) {
	product := roomCouncil("product", "ada", "ben", "cust1", "cust2")
	customers := roomCouncil("customers", "cust1", "cust2")
	risk := roomCouncil("risk", "cleo")

	tests := []struct {
		name      string
		councils  []Council
		want      map[string][]string // council name -> member ids
		wantNotes []string
	}{
		{
			name:     "shared members speak in the smaller council",
			councils: []Council{product, customers},
			want:     map[string][]string{"product": {"ada", "ben"}, "customers": {"cust1", "cust2"}},
			wantNotes: []string{
				"Virtual Cust1 sits in several of these councils and speaks only in the customers council",
				"Virtual Cust2 sits in several",
			},
		},
		{
			name:      "order doesn't matter",
			councils:  []Council{customers, product},
			want:      map[string][]string{"product": {"ada", "ben"}, "customers": {"cust1", "cust2"}},
			wantNotes: []string{"speaks only in the customers council"},
		},
		{
			name:      "ties go to the first listed; an emptied council is dropped",
			councils:  []Council{roomCouncil("a", "ada", "ben"), roomCouncil("b", "ada", "ben")},
			want:      map[string][]string{"a": {"ada", "ben"}},
			wantNotes: []string{"the b council has no one left to speak"},
		},
		{
			name:      "a council of one gets a warning",
			councils:  []Council{risk},
			want:      map[string][]string{"risk": {"cleo"}},
			wantNotes: []string{"the risk council has one member (Virtual Cleo)"},
		},
		{
			name:     "no overlap, no notes",
			councils: []Council{roomCouncil("a", "ada", "ben"), roomCouncil("b", "cleo", "dan")},
			want:     map[string][]string{"a": {"ada", "ben"}, "b": {"cleo", "dan"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, notes := SeatCouncils(tt.councils)
			got := map[string][]string{}
			for _, c := range out {
				for _, in := range c.Inputs {
					got[c.Name] = append(got[c.Name], in.Expert.ID)
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("seated = %v, want %v", got, tt.want)
			}
			joined := strings.Join(notes, "\n")
			for _, n := range tt.wantNotes {
				if !strings.Contains(joined, n) {
					t.Errorf("notes = %q, want one containing %q", notes, n)
				}
			}
			if len(tt.wantNotes) == 0 && len(notes) > 0 {
				t.Errorf("unexpected notes: %q", notes)
			}
		})
	}
}
