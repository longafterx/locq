package main

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func TestReduceLinesPreservesEvidence(t *testing.T) {
	t.Parallel()
	lines := []numberedLine{
		{original: 1, data: []byte("noise\n")}, {original: 2, data: []byte("OPEN\n")},
		{original: 3, data: []byte("noise\n")}, {original: 4, data: []byte("CLOSE\n")},
		{original: 5, data: []byte("noise\n")},
	}
	kept, minimal, probes, err := reduceLines(lines, 100, func(data []byte) (bool, error) {
		return bytes.Contains(data, []byte("OPEN")) && bytes.Contains(data, []byte("CLOSE")), nil
	})
	if err != nil || !minimal || probes == 0 {
		t.Fatalf("result=%v probes=%d err=%v", minimal, probes, err)
	}
	numbers := []int{}
	for _, line := range kept {
		numbers = append(numbers, line.original)
	}
	if !reflect.DeepEqual(numbers, []int{2, 4}) {
		t.Fatalf("lost original line attribution: %v", numbers)
	}
}

func TestReduceLinesBudgetAndFailure(t *testing.T) {
	t.Parallel()
	lines := []numberedLine{{original: 1, data: []byte("a\n")}, {original: 2, data: []byte("b\n")}}
	_, minimal, probes, err := reduceLines(lines, 1, func([]byte) (bool, error) { return false, nil })
	if err != nil || minimal || probes != 1 {
		t.Fatal("budget-limited result called minimal")
	}
	failure := errors.New("tool failed")
	kept, minimal, _, err := reduceLines(lines, 100, func([]byte) (bool, error) { return false, failure })
	if !errors.Is(err, failure) || minimal || len(kept) != 2 {
		t.Fatal("failed probe lost previous evidence")
	}
}
