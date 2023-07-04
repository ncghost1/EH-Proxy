package datastructure

import (
	"testing"
)

func TestTrie(t *testing.T) {
	trie := NewTrie()
	trie.Insert("apple")
	trie.Insert("banana")

	res := trie.Search("apple")
	if !res {
		t.Errorf("expect:%v, actual:%v", true, res)
	}
	res = trie.Search("banana")
	if !res {
		t.Errorf("expect:%v, actual:%v", true, res)
	}
	res = trie.Search("orange")
	if res {
		t.Errorf("expect:%v, actual:%v", false, res)
	}
	res = trie.PrefixSearch("applelele")
	if !res {
		t.Errorf("expect:%v, actual:%v", true, res)
	}
	res = trie.PrefixSearch("appl")
	if res {
		t.Errorf("expect:%v, actual:%v", false, res)
	}
}
