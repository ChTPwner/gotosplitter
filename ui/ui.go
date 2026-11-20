// Package ui implements the user interface for GotoSplitter, allowing users to select game definitions and reset the timer.
package ui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"goto_splitter/config"
)

type (
	DefinitionSelectionHandler func(folder, filename string) error
	ResetHandler               func(saveAttempt, resetSNES bool) error
	StatusUpdateHandler        func(sniStatus, wsStatus string)
)

type resetChoice uint8

const (
	resetSaveAttempt resetChoice = iota
	resetDiscardAttempt
	resetCancel
)

func ShowStartupError(ctx context.Context, startupErr error) error {
	if startupErr == nil {
		return fmt.Errorf("startup error is required")
	}

	a := app.New()
	w := a.NewWindow("GotoSplitter startup error")
	message := widget.NewLabel(fmt.Sprintf("GotoSplitter could not start:\n\n%v", startupErr))
	message.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewVBox(message, widget.NewButton("Quit", a.Quit)))
	w.Resize(fyne.NewSize(640, 220))

	var closeOnce sync.Once
	closed := make(chan struct{})
	w.SetOnClosed(func() {
		closeOnce.Do(func() { close(closed) })
	})
	go func() {
		select {
		case <-ctx.Done():
			a.Quit()
		case <-closed:
		}
	}()
	w.ShowAndRun()
	return nil
}

func RunUI(ctx context.Context, cfg *config.Config, startupErr error, onDefinitionChange DefinitionSelectionHandler, onReset ResetHandler, onStatusUpdate StatusUpdateHandler) error {
	if cfg == nil {
		return fmt.Errorf("configuration is required")
	}
	if cfg.GameDefinitionFolder == "" {
		folder, err := config.DefaultGameDefinitionFolder()
		if err != nil {
			return err
		}
		cfg.GameDefinitionFolder = folder
	}

	a := app.New()
	w := a.NewWindow("GotoSplitter")
	folderEntry := widget.NewEntry()
	folderEntry.SetText(cfg.GameDefinitionFolder)
	folderEntry.SetPlaceHolder("Select a folder containing JSON definitions")
	definitionSelect := widget.NewSelect(nil, nil)
	status := widget.NewLabel("")
	if startupErr != nil {
		status.SetText(formatStartupDefinitionError(startupErr))
	}

	sniStatus := widget.NewLabel("Disconnected")
	wsStatus := widget.NewLabel("Waiting for LiveSplit One...")

	currentFolder := cfg.GameDefinitionFolder
	currentDefinition := cfg.CurrentGameDefinition
	updatingSelection := false
	applySelection := func(folder, filename string) error {
		if onDefinitionChange != nil {
			if err := onDefinitionChange(folder, filename); err != nil {
				return err
			}
		} else {
			next := *cfg
			next.GameDefinitionFolder = folder
			next.CurrentGameDefinition = filename
			if err := config.Save(&next); err != nil {
				return err
			}
			*cfg = next
		}
		currentFolder = folder
		currentDefinition = filename
		status.SetText("")
		return nil
	}
	refreshDefinitions := func(folder string) (string, error) {
		names, err := config.ListGameDefinitions(folder)
		if err != nil {
			return "", err
		}
		updatingSelection = true
		definitionSelect.Options = names
		definitionSelect.Refresh()
		selected := chooseDefinition(names, currentDefinition)
		definitionSelect.SetSelected(selected)
		updatingSelection = false
		return selected, nil
	}
	onFolderSelected := func(folder string) {
		if folder == "" {
			return
		}
		folderEntry.SetText(folder)
		selected, err := refreshDefinitions(folder)
		if err != nil {
			dialog.ShowError(err, w)
			folderEntry.SetText(currentFolder)
			return
		}
		if err := applySelection(folder, selected); err != nil {
			dialog.ShowError(err, w)
			status.SetText(err.Error())
			folderEntry.SetText(currentFolder)
			if _, refreshErr := refreshDefinitions(currentFolder); refreshErr != nil {
				status.SetText(refreshErr.Error())
			}
			return
		}
	}

	if _, err := refreshDefinitions(currentFolder); err != nil {
		status.SetText(err.Error())
	}
	definitionSelect.OnChanged = func(filename string) {
		if updatingSelection {
			return
		}
		if err := applySelection(currentFolder, filename); err != nil {
			dialog.ShowError(err, w)
			status.SetText(err.Error())
			updatingSelection = true
			definitionSelect.SetSelected(currentDefinition)
			updatingSelection = false
		}
	}
	folderEntry.OnSubmitted = onFolderSelected

	browseButton := widget.NewButton("Browse", func() {
		d := dialog.NewFolderOpen(func(folder fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if folder != nil {
				onFolderSelected(folder.Path())
			}
		}, w)
		d.Show()
	})
	applyFolderButton := widget.NewButton("Use folder", func() {
		onFolderSelected(folderEntry.Text)
	})

	resetButton := widget.NewButton("Reset timer", func() {
		if onReset == nil {
			dialog.ShowError(fmt.Errorf("reset handler is unavailable"), w)
			return
		}

		var resetDialog *dialog.CustomDialog
		resetGameCheck := widget.NewCheck("Also reset the SNES game", nil)
		resetGameCheck.SetChecked(cfg.ResetSNESOnTimerReset)
		resetDialog = dialog.NewCustomWithoutButtons(
			"Reset timer",
			container.NewVBox(
				widget.NewLabel("Save this attempt to history before resetting?"),
				resetGameCheck,
			),
			w,
		)
		resetDialog.SetButtons(resetChoiceButtons(func(choice resetChoice) {
			resetDialog.Hide()
			if choice == resetCancel {
				return
			}
			resetSNES := resetGameCheck.Checked
			cfg.ResetSNESOnTimerReset = resetSNES
			resetConfig := *cfg
			saveErr := saveResetPreference(resetConfig, config.Save)
			go func() {
				if err := applyConfirmedReset(choice, resetSNES, saveErr, onReset); err != nil {
					fyne.Do(func() {
						dialog.ShowError(err, w)
					})
				}
			}()
		}))
		resetDialog.Show()
	})
	if onReset == nil {
		resetButton.Disable()
	}

	w.SetContent(container.NewVBox(
		widget.NewLabel("Game definition folder"),
		container.NewBorder(nil, nil, nil, browseButton, folderEntry),
		applyFolderButton,
		widget.NewLabel("Current game definition"),
		definitionSelect,
		resetButton,
		widget.NewLabel("Status"),
		status,
		widget.NewLabel("SNI Status"),
		sniStatus,
		widget.NewLabel("WebSocket Status"),
		wsStatus,
	))
	w.Resize(fyne.NewSize(640, 300))

	var closeOnce sync.Once
	closed := make(chan struct{})
	w.SetOnClosed(func() {
		closeOnce.Do(func() { close(closed) })
	})
	go func() {
		select {
		case <-ctx.Done():
			a.Quit()
		case <-closed:
		}
	}()
	w.ShowAndRun()
	return nil
}

func chooseDefinition(names []string, current string) string {
	for _, name := range names {
		if name == current {
			return current
		}
	}
	if len(names) != 0 {
		return names[0]
	}
	return ""
}

func formatStartupDefinitionError(startupErr error) string {
	if startupErr == nil {
		return ""
	}
	return fmt.Sprintf("Game definition error: %v", startupErr)
}

func resetChoiceButtons(onChoice func(resetChoice)) []fyne.CanvasObject {
	return []fyne.CanvasObject{
		widget.NewButton("Save attempt & reset", func() { onChoice(resetSaveAttempt) }),
		widget.NewButton("Discard attempt", func() { onChoice(resetDiscardAttempt) }),
		widget.NewButton("Cancel", func() { onChoice(resetCancel) }),
	}
}

func applyResetChoice(choice resetChoice, resetSNES bool, onReset ResetHandler) error {
	switch choice {
	case resetCancel:
		return nil
	case resetSaveAttempt, resetDiscardAttempt:
		if onReset == nil {
			return fmt.Errorf("reset handler is unavailable")
		}
		return onReset(choice == resetSaveAttempt, resetSNES)
	default:
		return fmt.Errorf("invalid reset choice %d", choice)
	}
}

func saveResetPreference(cfg config.Config, saveConfig func(*config.Config) error) error {
	if saveConfig == nil {
		return fmt.Errorf("configuration save handler is unavailable")
	}
	if err := saveConfig(&cfg); err != nil {
		return fmt.Errorf("save SNES reset preference: %w", err)
	}
	return nil
}

func combineResetErrors(saveErr, resetErr error) error {
	if resetErr != nil {
		resetErr = fmt.Errorf("reset run: %w", resetErr)
	}
	return errors.Join(saveErr, resetErr)
}

func applyConfirmedReset(choice resetChoice, resetSNES bool, saveErr error, onReset ResetHandler) error {
	return combineResetErrors(saveErr, applyResetChoice(choice, resetSNES, onReset))
}
