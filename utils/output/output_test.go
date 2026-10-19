package output

import (
	"bytes"
	"errors"
	"testing"
)

// newOutput returns an Output writing to buffers, and the buffers.
func newOutput() (Output, *bytes.Buffer, *bytes.Buffer) {
	var out, err bytes.Buffer
	return Output{Out: &out, Err: &err}, &out, &err
}

func TestMessages(t *testing.T) {
	cases := []struct {
		name    string
		write   func(o Output)
		wantOut string
		wantErr string
	}{
		{
			name:    "Infof writes to Out as given",
			write:   func(o Output) { o.Infof("built %d pages\n", 3) },
			wantOut: "built 3 pages\n",
		},
		{
			name:    "Warnf writes a coloured warning line to Err",
			write:   func(o Output) { o.Warnf("%s is a draft", "about.md") },
			wantErr: Warning + "warning: about.md is a draft" + Reset + "\n",
		},
		{
			name:    "Errorf writes an error line to Err",
			write:   func(o Output) { o.Errorf("%s has no date", "post.md") },
			wantErr: "error: post.md has no date\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, out, err := newOutput()
			c.write(o)
			if out.String() != c.wantOut || err.String() != c.wantErr {
				t.Errorf("wrote (out %q, err %q), want (out %q, err %q)", out, err, c.wantOut, c.wantErr)
			}
		})
	}
}

func TestStep(t *testing.T) {
	t.Run("success ends the line with Done", func(t *testing.T) {
		o, out, _ := newOutput()
		if err := o.Step("Building...", func() error { return nil }); err != nil {
			t.Fatal(err)
		}
		if want := "Building... Done! :)\n"; out.String() != want {
			t.Errorf("wrote %q, want %q", out, want)
		}
	})

	t.Run("failure returns the error and ends the line", func(t *testing.T) {
		o, out, _ := newOutput()
		taskErr := errors.New("task failed")
		if err := o.Step("Building...", func() error { return taskErr }); !errors.Is(err, taskErr) {
			t.Errorf("Step returned %v, want %v", err, taskErr)
		}
		if want := "Building... \n"; out.String() != want {
			t.Errorf("wrote %q, want %q", out, want)
		}
	})
}
