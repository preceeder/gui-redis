package keytree

import (
	"slices"
	"testing"
)

func TestBuildPrefixTree(t *testing.T) {
	tree := Build([]string{"user:1:name", "user:1:email", "user:2:name", "session:abc", "plain"}, ":")
	if !slices.Equal(tree.Roots, []string{"plain", "session", "user"}) {
		t.Fatalf("roots %v", tree.Roots)
	}
	if !tree.IsKey["plain"] || tree.IsKey["user"] || !tree.IsKey["user:1:name"] {
		t.Fatalf("isKey %+v", tree.IsKey)
	}
	if !slices.Equal(tree.Kids["user"], []string{"user:1", "user:2"}) {
		t.Fatalf("user kids %v", tree.Kids["user"])
	}
	if !slices.Equal(tree.Kids["user:1"], []string{"user:1:email", "user:1:name"}) {
		t.Fatalf("user:1 kids %v", tree.Kids["user:1"])
	}
	if tree.Label["user:1:email"] != "email" {
		t.Fatalf("label %q", tree.Label["user:1:email"])
	}
	if tree.ChildrenOf("plain") != nil {
		t.Fatal("leaf should have nil children")
	}
	if n := tree.DescendantKeys("user"); n != 3 {
		t.Fatalf("descendants %d", n)
	}
}

func TestBuildKeyThatIsAlsoPrefix(t *testing.T) {
	tree := Build([]string{"user", "user:1"}, ":")
	if !tree.IsKey["user"] || !tree.IsKey["user:1"] {
		t.Fatal(tree.IsKey)
	}
	if !slices.Equal(tree.Kids["user"], []string{"user:1"}) {
		t.Fatalf("kids %v", tree.Kids["user"])
	}
}

func TestEmptySeparatorIsFlat(t *testing.T) {
	tree := Build([]string{"a:b", "c"}, "")
	if !slices.Equal(tree.Roots, []string{"a:b", "c"}) {
		t.Fatalf("roots %v", tree.Roots)
	}
	if tree.ChildrenOf("a:b") != nil {
		t.Fatal("expected leaves")
	}
}
