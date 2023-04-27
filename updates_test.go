package main

import (
	"fmt"
	"testing"
)

func TestIsAdminUpdate(t *testing.T) {
	var tests = []struct {
		cmd  string
		want bool
	}{
		{"admin_something", true},
		{"adminsomething", false},
		{"admi_something", false},
		{"something_admin", false},
		{"admin", true},
		{"word_admin_word", false},
	}
	for _, tt := range tests {
		testAdminCmd := fmt.Sprintf("%s", tt.cmd)
		t.Run(testAdminCmd, func(t *testing.T) {
			ans := isAdminCommand(tt.cmd)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}
