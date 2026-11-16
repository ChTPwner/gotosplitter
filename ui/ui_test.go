package ui

import "testing"

func TestApplyResetChoice(t *testing.T) {
	tests := []struct {
		name       string
		choice     resetChoice
		wantCalled bool
		wantSave   bool
	}{
		{name: "save", choice: resetSaveAttempt, wantCalled: true, wantSave: true},
		{name: "discard", choice: resetDiscardAttempt, wantCalled: true, wantSave: false},
		{name: "cancel", choice: resetCancel, wantCalled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			err := applyResetChoice(tt.choice, func(saveAttempt bool) error {
				called = true
				if saveAttempt != tt.wantSave {
					t.Fatalf("saveAttempt = %t, want %t", saveAttempt, tt.wantSave)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("apply reset choice: %v", err)
			}
			if called != tt.wantCalled {
				t.Fatalf("handler called = %t, want %t", called, tt.wantCalled)
			}
		})
	}
}
