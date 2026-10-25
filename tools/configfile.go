package tools

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// errConfigNotFound is returned by loadConfigFile when no search directory holds the file.
var errConfigNotFound = errors.New("config file not found")

// configSearchDirs is where config.yaml and branding.yaml are looked for, first hit wins.
func configSearchDirs() []string {
	return []string{"/etc/TUM-Live", os.ExpandEnv("$HOME/.TUM-Live"), "."}
}

// findConfigFile returns the path of <name>.yaml (or .yml) in the first search directory that has one.
func findConfigFile(name string) (string, error) {
	for _, dir := range configSearchDirs() {
		for _, ext := range []string{".yaml", ".yml"} {
			path := filepath.Join(dir, name+ext)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s.yaml in %s", errConfigNotFound, name, strings.Join(configSearchDirs(), ", "))
}

// loadConfigFile decodes the named config file into out and returns the path it read.
// Fields the file omits keep their value in out, so callers can preset defaults.
func loadConfigFile(name string, out any) (string, error) {
	path, err := findConfigFile(name)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return path, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return path, err
	}
	if doc.Kind == 0 { // empty file
		return path, nil
	}
	normalizeNode(&doc, reflect.TypeOf(out))
	return path, doc.Decode(out)
}

// setConfigKey sets a top-level key in the YAML file at path, leaving comments and the
// other keys untouched. The key is matched case-insensitively, since the config format
// has always accepted any casing (config.yaml spells it "jwtkey").
func setConfigKey(path, key, value string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: top level is not a mapping", path)
	}
	val := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	if strings.Contains(value, "\n") {
		val.Style = yaml.LiteralStyle
	}
	found := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if strings.EqualFold(root.Content[i].Value, key) {
			root.Content[i+1] = val
			found = true
			break
		}
	}
	if !found {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

// normalizeNode makes the YAML tree decodable into t the way the config files have always
// been read: keys match case-insensitively by yaml tag or field name ("camauth",
// "rootURLs", "voiceservice"), and a quoted scalar is accepted for a number or bool field
// ("port: \"3306\""). yaml.v3 alone is strict about both.
func normalizeNode(n *yaml.Node, t reflect.Type) {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if n == nil || t == nil {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			normalizeNode(c, t)
		}
	case yaml.SequenceNode:
		if t.Kind() == reflect.Slice {
			for _, c := range n.Content {
				normalizeNode(c, t.Elem())
			}
		}
	case yaml.ScalarNode:
		if n.Tag == "!!str" && isNumberOrBool(t.Kind()) && resolvesTo(n.Value, t.Kind()) {
			n.Tag, n.Style = "", 0 // let the decoder infer the type from the text
		}
	case yaml.MappingNode:
		if t.Kind() != reflect.Struct {
			return
		}
		fields := structKeys(t)
		for i := 0; i+1 < len(n.Content); i += 2 {
			if f, ok := fields[strings.ToLower(n.Content[i].Value)]; ok {
				n.Content[i].Value = f.key
				normalizeNode(n.Content[i+1], f.typ)
			}
		}
	}
}

type structKey struct {
	key string
	typ reflect.Type
}

// structKeys maps every lower-cased spelling a field answers to (tag name and Go field
// name) to the key yaml.v3 expects for it.
func structKeys(t reflect.Type) map[string]structKey {
	keys := make(map[string]structKey, t.NumField()*2)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name) // yaml.v3's default for untagged fields
		}
		sk := structKey{key: name, typ: f.Type}
		keys[strings.ToLower(f.Name)] = sk
		keys[strings.ToLower(name)] = sk
	}
	return keys
}

func isNumberOrBool(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func resolvesTo(s string, k reflect.Kind) bool {
	var err error
	switch k {
	case reflect.Bool:
		_, err = strconv.ParseBool(s)
	case reflect.Float32, reflect.Float64:
		_, err = strconv.ParseFloat(s, 64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		_, err = strconv.ParseUint(s, 10, 64)
	default:
		_, err = strconv.ParseInt(s, 10, 64)
	}
	return err == nil
}
