package keytree

import (
	"sort"
	"strings"
)

// Tree is a prefix tree of Redis keys.
type Tree struct {
	Roots []string
	Kids  map[string][]string
	IsKey map[string]bool
	Label map[string]string
}

// Build groups keys by sep. An empty sep makes every key a root.
func Build(keys []string, sep string) Tree {
	t := Tree{
		Kids:  map[string][]string{},
		IsKey: map[string]bool{},
		Label: map[string]string{},
	}
	seenKid := map[string]map[string]struct{}{}
	addKid := func(parent, child string) {
		if parent == child {
			return
		}
		if seenKid[parent] == nil {
			seenKid[parent] = map[string]struct{}{}
		}
		if _, ok := seenKid[parent][child]; ok {
			return
		}
		seenKid[parent][child] = struct{}{}
		t.Kids[parent] = append(t.Kids[parent], child)
	}

	rootSet := map[string]struct{}{}
	for _, key := range keys {
		if key == "" {
			continue
		}
		t.IsKey[key] = true
		if sep == "" || !strings.Contains(key, sep) {
			rootSet[key] = struct{}{}
			t.Label[key] = key
			continue
		}
		parts := strings.Split(key, sep)
		path := ""
		for i, part := range parts {
			parent := path
			if i == 0 {
				path = part
			} else {
				path = path + sep + part
			}
			t.Label[path] = part
			if i == 0 {
				rootSet[path] = struct{}{}
			} else {
				addKid(parent, path)
			}
		}
	}

	t.Roots = keysOf(rootSet)
	sort.Strings(t.Roots)
	for parent, kids := range t.Kids {
		sort.Strings(kids)
		t.Kids[parent] = kids
	}
	return t
}

// ChildrenOf returns kids, or nil when the node is a leaf (so Outline hides the arrow).
func (t Tree) ChildrenOf(id string) []string {
	kids := t.Kids[id]
	if len(kids) == 0 {
		return nil
	}
	return kids
}

// DescendantKeys counts keys under id, including id itself when it is a key.
func (t Tree) DescendantKeys(id string) int {
	n := 0
	var walk func(string)
	walk = func(cur string) {
		if t.IsKey[cur] {
			n++
		}
		for _, k := range t.Kids[cur] {
			walk(k)
		}
	}
	walk(id)
	return n
}

func keysOf(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}
