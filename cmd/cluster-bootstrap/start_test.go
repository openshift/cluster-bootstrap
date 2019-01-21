package main

import (
	"reflect"
	"testing"
)

func Test_parsePodPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		clauses  []string
		expected map[string][]string
		wantErr  bool
	}{
		{"empty", []string{}, map[string][]string{}, false},
		{"nil", nil, map[string][]string{}, false},
		{"no-colon", []string{"foo/bar"}, map[string][]string{"foo/bar": {"foo/bar"}}, false},
		{"colon", []string{"desc:foo/bar"}, map[string][]string{"desc": {"foo/bar"}}, false},
		{"no-namespace", []string{"foo"}, map[string][]string{"foo": {"foo"}}, false},
		{"disjunction", []string{"desc:foo/bar|abc/def|ghi"}, map[string][]string{"desc": {"foo/bar", "abc/def", "ghi"}}, false},
		{"disjunction,no-desc", []string{"foo/bar|abc/def"}, nil, true},
		{"multiple-disjunctions", []string{"desc:foo/bar|abc/def|ghi", "desc2:jkl/mno"}, map[string][]string{"desc": {"foo/bar", "abc/def", "ghi"}, "desc2": {"jkl/mno"}}, false},
		{"mixed", []string{"desc:foo/bar", "abc/def"}, map[string][]string{"desc": {"foo/bar"}, "abc/def": {"abc/def"}}, false},
		{"split-disjunctions", []string{"desc:foo/bar|abc/def|ghi", "desc:jkl/mno"}, map[string][]string{"desc": {"foo/bar", "abc/def", "ghi", "jkl/mno"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePodPrefixes(tt.clauses)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePodPrefixes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parsePodPrefixes() = %v, want %v", got, tt.expected)
			}
		})
	}
}
