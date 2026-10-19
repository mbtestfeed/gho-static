package yaml_edit

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// EditFile calls edit with the top level mapping in the yaml file at path
// (empty if it's missing or empty), writing the file back if edit reports a
// change, keeping the rest of it, including comments and order, as it is.
func EditFile(path string, edit func(root *yaml.Node) (bool, error)) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("can't change %s: it isn't a list of settings", path)
	}
	if changed, err := edit(root); err != nil || !changed {
		return err
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// MappingValue returns the value of key in the yaml mapping m, or nil if
// it has none.
func MappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
