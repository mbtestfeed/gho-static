package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"

	"gho-static/utils/output"
)

// newTerminal returns a Terminal answering with input, and the buffer its
// messages are written to.
func newTerminal(input string, interactive bool) (Terminal, *bytes.Buffer) {
	var out bytes.Buffer
	return Terminal{
		In:          bufio.NewReader(strings.NewReader(input)),
		Output:      output.Output{Out: &out, Err: &out},
		Interactive: interactive,
	}, &out
}

func TestAsk(t *testing.T) {
	term, out := newTerminal("  answer  \n", true)
	if got := term.Ask("Question? ", []string{"a hint"}); got != "answer" {
		t.Errorf("Ask = %q, want %q", got, "answer")
	}
	for _, want := range []string{"Question? ", output.Grey + "a hint" + output.Reset} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output doesn't contain %q:\n%q", want, out.String())
		}
	}
}

func TestAskSecret(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		interactive bool
		hidden      string // read without showing it, if set
		want        string
	}{
		{name: "interactive reads hidden", input: "shown\n", interactive: true, hidden: "hidden", want: "hidden"},
		{name: "non-interactive reads input", input: "shown\n", hidden: "hidden", want: "shown"},
		{name: "interactive without hidden reading reads input", input: "shown\n", interactive: true, want: "shown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			term, _ := newTerminal(c.input, c.interactive)
			if c.hidden != "" {
				term.readHidden = func() ([]byte, error) { return []byte(c.hidden), nil }
			}
			if got := term.AskSecret("Secret: ", nil); got != c.want {
				t.Errorf("AskSecret = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAskSetting(t *testing.T) {
	for _, secret := range []bool{false, true} {
		term, _ := newTerminal("shown\n", true)
		term.readHidden = func() ([]byte, error) { return []byte("hidden"), nil }
		want := "shown"
		if secret {
			want = "hidden"
		}
		if got := term.AskSetting("Setting: ", nil, secret); got != want {
			t.Errorf("AskSetting(secret %v) = %q, want %q", secret, got, want)
		}
	}
}

func TestAskDate(t *testing.T) {
	today := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name        string
		input       string
		want        time.Time
		wantInvalid bool // whether an answer is said to be invalid
	}{
		{name: "enter gives today", input: "\n", want: today},
		{name: "date given", input: "20260102\n", want: time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)},
		{name: "asks again after an invalid date", input: "2026-01-02\n20260103\n", want: time.Date(2026, 1, 3, 0, 0, 0, 0, time.Local), wantInvalid: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			term, out := newTerminal(c.input, true)
			if got := term.AskDate("post.md", today); !got.Equal(c.want) {
				t.Errorf("AskDate = %v, want %v", got, c.want)
			}
			if invalid := strings.Contains(out.String(), "isn't a valid date"); invalid != c.wantInvalid {
				t.Errorf("said an answer was invalid: %v, want %v", invalid, c.wantInvalid)
			}
		})
	}
}

func TestYesNoQuestions(t *testing.T) {
	cases := []struct {
		name  string
		ask   func(Terminal) bool
		input string
		want  bool
	}{
		{"publish draft defaults to no", func(t Terminal) bool { return t.AskToPublishDraft("a.md") }, "\n", false},
		{"publish draft yes", func(t Terminal) bool { return t.AskToPublishDraft("a.md") }, "Y\n", true},
		{"publish draft asks again", func(t Terminal) bool { return t.AskToPublishDraft("a.md") }, "maybe\nyes\n", true},
		{"nav link defaults to yes", func(t Terminal) bool { return t.askNavLink("about") }, "\n", true},
		{"nav link no", func(t Terminal) bool { return t.askNavLink("about") }, "No\n", false},
		{"nav link asks again", func(t Terminal) bool { return t.askNavLink("about") }, "maybe\nn\n", false},
		{"new collection defaults to no", func(t Terminal) bool { return t.askNewCollection("news") }, "\n", false},
		{"new collection yes", func(t Terminal) bool { return t.askNewCollection("news") }, "yes\n", true},
		{"new collection asks again", func(t Terminal) bool { return t.askNewCollection("news") }, "maybe\ny\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			term, _ := newTerminal(c.input, true)
			if got := c.ask(term); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestAskDeploymentMethod(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"\n", "none"},
		{"1\n", "github-pages"},
		{"2\n", "cloudflare-pages"},
		{"cloudflare-pages\n", "cloudflare-pages"},
		{"GitHub Pages\n", "github-pages"},
		{"9\nbogus\nnone\n", "none"},
	}
	for _, c := range cases {
		term, out := newTerminal(c.input, true)
		if got := term.askDeploymentMethod(); got.Name != c.want {
			t.Errorf("askDeploymentMethod with %q = %q, want %q", c.input, got.Name, c.want)
		}
		if !strings.Contains(out.String(), "3: none (default)") {
			t.Errorf("the methods weren't listed with the default marked:\n%q", out.String())
		}
	}
}
