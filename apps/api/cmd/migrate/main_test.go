package main

import (
	"reflect"
	"testing"
)

func TestMigrationCommandPreservesGooseArguments(t *testing.T) {
	tests := []struct {
		input       []string
		wantCommand string
		wantArgs    []string
	}{
		{wantCommand: "up"},
		{input: []string{"status"}, wantCommand: "status"},
		{input: []string{"down-to", "20260713000100"}, wantCommand: "down-to", wantArgs: []string{"20260713000100"}},
	}
	for _, test := range tests {
		command, args := migrationCommand(test.input)
		if command != test.wantCommand || !reflect.DeepEqual(args, test.wantArgs) {
			t.Fatalf("migrationCommand(%v) = %q/%v, want %q/%v", test.input, command, args, test.wantCommand, test.wantArgs)
		}
	}
}
