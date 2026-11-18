package ui

import (
	"errors"
	"strings"
	"testing"

	"goto_splitter/config"
)

func TestApplyResetChoice(t *testing.T) {
	tests := []struct {
		name       string
		choice     resetChoice
		wantCalled bool
		wantSave   bool
		resetSNES  bool
	}{
		{name: "save", choice: resetSaveAttempt, wantCalled: true, wantSave: true},
		{name: "discard and reset SNES", choice: resetDiscardAttempt, wantCalled: true, resetSNES: true},
		{name: "cancel", choice: resetCancel, resetSNES: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			err := applyResetChoice(tt.choice, tt.resetSNES, func(saveAttempt, resetSNES bool) error {
				called = true
				if saveAttempt != tt.wantSave {
					t.Fatalf("saveAttempt = %t, want %t", saveAttempt, tt.wantSave)
				}
				if resetSNES != tt.resetSNES {
					t.Fatalf("resetSNES = %t, want %t", resetSNES, tt.resetSNES)
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

func TestSaveResetPreference(t *testing.T) {
	cfg := config.Config{ResetSNESOnTimerReset: true}
	saved := false
	err := saveResetPreference(cfg, func(got *config.Config) error {
		saved = true
		if !got.ResetSNESOnTimerReset {
			t.Fatal("saved preference = false, want true")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("save reset preference: %v", err)
	}
	if !saved {
		t.Fatal("save callback was not called")
	}
}

func TestApplyConfirmedResetDoesNothingOnCancel(t *testing.T) {
	err := applyConfirmedReset(resetCancel, true, nil, func(bool, bool) error {
		t.Fatal("reset callback called on cancel")
		return nil
	})
	if err != nil {
		t.Fatalf("cancel reset: %v", err)
	}
}

func TestApplyConfirmedResetContinuesAfterPreferenceSaveFailure(t *testing.T) {
	resetCalled := false
	err := applyConfirmedReset(resetDiscardAttempt, true, errors.New("preference disk full"), func(saveAttempt, resetSNES bool) error {
		resetCalled = true
		if saveAttempt || !resetSNES {
			t.Fatalf("reset options = save:%t SNES:%t, want save:false SNES:true", saveAttempt, resetSNES)
		}
		return nil
	})
	if !resetCalled {
		t.Fatal("reset callback was not called after preference save failure")
	}
	if err == nil || !strings.Contains(err.Error(), "preference disk full") {
		t.Fatalf("combined error = %v, want preference save failure", err)
	}
}

func TestApplyConfirmedResetReportsBothFailures(t *testing.T) {
	err := applyConfirmedReset(resetDiscardAttempt, true, errors.New("preference disk full"), func(bool, bool) error {
		return errors.New("LiveSplit unavailable")
	})
	if err == nil || !strings.Contains(err.Error(), "preference disk full") || !strings.Contains(err.Error(), "LiveSplit unavailable") {
		t.Fatalf("combined error = %v, want both preference and reset failures", err)
	}
}

func TestSaveResetPreferenceReportsFailure(t *testing.T) {
	err := saveResetPreference(config.Config{}, func(*config.Config) error {
		return errors.New("disk full")
	})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("save preference error = %v, want disk full", err)
	}
}
