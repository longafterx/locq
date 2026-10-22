package core

import (
	"testing"
)

func TestContainsComment(t *testing.T) {
	if !containsCommentBytes([]byte(`int a; /* A takes care of counts */`), [][]string{{"/*", "*/"}}) {
		t.Errorf("invalid")
	}
	if !containsCommentBytes([]byte(`bool f; /* `), [][]string{{"/*", "*/"}}) {
		t.Errorf("invalid")
	}
	if containsCommentBytes([]byte(`}`), [][]string{{"/*", "*/"}}) {
		t.Errorf("invalid")
	}
}
