package yaml_edit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// setValue returns an edit setting key to value in the top level mapping,
// adding it if missing.
func setValue(key, value string) func(root *yaml.Node) (bool, error) {
	return func(root *yaml.Node) (bool, error) {
		if n := MappingValue(root, key); n != nil {
			n.Value = value
			return true, nil
		}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Value: value})
		return true, nil
	}
}

// writeYAML writes content to a yaml file in a new temporary folder,
// returning its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// readYAML returns the contents of path, failing the test if it can't be read.
func readYAML(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEditFile(t *testing.T) {
	t.Run("missing file is created", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.yaml")
		if err := EditFile(path, setValue("name", "site")); err != nil {
			t.Fatal(err)
		}
		if got := readYAML(t, path); got != "name: site\n" {
			t.Errorf("file = %q, want \"name: site\\n\"", got)
		}
	})

	t.Run("keeps comments and order", func(t *testing.T) {
		path := writeYAML(t, "# top comment\nname: old # inline comment\nother: 1\n")
		if err := EditFile(path, setValue("name", "new")); err != nil {
			t.Fatal(err)
		}
		got := readYAML(t, path)
		for _, want := range []string{"# top comment", "name: new # inline comment", "other: 1"} {
			if !strings.Contains(got, want) {
				t.Errorf("file = %q, want it to contain %q", got, want)
			}
		}
		if strings.Index(got, "name:") > strings.Index(got, "other:") {
			t.Errorf("file = %q, want name still before other", got)
		}
	})

	t.Run("unchanged file isn't rewritten", func(t *testing.T) {
		content := "name:    spaced   \n"
		path := writeYAML(t, content)
		if err := EditFile(path, func(*yaml.Node) (bool, error) { return false, nil }); err != nil {
			t.Fatal(err)
		}
		if got := readYAML(t, path); got != content {
			t.Errorf("file = %q, want it untouched as %q", got, content)
		}
	})

	t.Run("edit error is returned and file left alone", func(t *testing.T) {
		content := "name: old\n"
		path := writeYAML(t, content)
		editErr := errors.New("edit failed")
		if err := EditFile(path, func(*yaml.Node) (bool, error) { return true, editErr }); !errors.Is(err, editErr) {
			t.Errorf("EditFile returned %v, want %v", err, editErr)
		}
		if got := readYAML(t, path); got != content {
			t.Errorf("file = %q, want it untouched as %q", got, content)
		}
	})

	t.Run("top level list is refused", func(t *testing.T) {
		path := writeYAML(t, "- a\n- b\n")
		if err := EditFile(path, setValue("name", "site")); err == nil {
			t.Error("EditFile on a list returned no error")
		}
	})

	t.Run("invalid yaml is refused", func(t *testing.T) {
		path := writeYAML(t, "name: [unclosed\n")
		if err := EditFile(path, setValue("name", "site")); err == nil {
			t.Error("EditFile on invalid yaml returned no error")
		}
	})
}

func TestMappingValue(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("a: 1\nb: [x, y]\n"), &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]

	if got := MappingValue(root, "a"); got == nil || got.Value != "1" {
		t.Errorf("MappingValue(a) = %v, want the scalar 1", got)
	}
	if got := MappingValue(root, "b"); got == nil || got.Kind != yaml.SequenceNode {
		t.Errorf("MappingValue(b) = %v, want a sequence", got)
	}
	if got := MappingValue(root, "missing"); got != nil {
		t.Errorf("MappingValue(missing) = %v, want nil", got)
	}
	if got := MappingValue(&yaml.Node{Kind: yaml.MappingNode}, "a"); got != nil {
		t.Errorf("MappingValue on an empty mapping = %v, want nil", got)
	}
}
