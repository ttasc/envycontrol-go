package main

import (
	"testing"
)

func TestParseArgsUpdateFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantUpdate  bool
		wantErr     bool
	}{
		{"Long update flag", []string{"envycontrol", "--update"}, true, false},
		{"Short update flag", []string{"envycontrol", "-u"}, true, false},
		{"Query flag (no update)", []string{"envycontrol", "-q"}, false, false},
		{"Invalid flag", []string{"envycontrol", "--super-update"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseArgsInternal(tt.args)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgsInternal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && opts.Update != tt.wantUpdate {
				t.Errorf("Expected Update=%v, got %v", tt.wantUpdate, opts.Update)
			}
		})
	}
}
