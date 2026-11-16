package ui

import (
	"context"
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"goto_splitter/config"
)

type DefinitionSelectionHandler func(folder, filename string) error

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

func RunUI(ctx context.Context, cfg *config.Config, onDefinitionChange DefinitionSelectionHandler) error {
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
	refreshDefinitions := func(folder string) error {
		names, err := config.ListGameDefinitions(folder)
		if err != nil {
			return err
		}
		updatingSelection = true
		definitionSelect.Options = names
		definitionSelect.Refresh()
		selected := currentDefinition
		found := false
		for _, name := range names {
			if name == selected {
				found = true
				break
			}
		}
		if !found {
			selected = ""
			if len(names) != 0 {
				selected = names[0]
			}
		}
		definitionSelect.SetSelected(selected)
		updatingSelection = false
		return nil
	}
	onFolderSelected := func(folder string) {
		if folder == "" {
			return
		}
		folderEntry.SetText(folder)
		if err := refreshDefinitions(folder); err != nil {
			dialog.ShowError(err, w)
			folderEntry.SetText(currentFolder)
			return
		}
		if err := applySelection(folder, definitionSelect.Selected); err != nil {
			dialog.ShowError(err, w)
			status.SetText(err.Error())
			folderEntry.SetText(currentFolder)
			if refreshErr := refreshDefinitions(currentFolder); refreshErr != nil {
				status.SetText(refreshErr.Error())
			}
			return
		}
	}

	if err := refreshDefinitions(currentFolder); err != nil {
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

	w.SetContent(container.NewVBox(
		widget.NewLabel("Game definition folder"),
		container.NewBorder(nil, nil, nil, browseButton, folderEntry),
		applyFolderButton,
		widget.NewLabel("Current game definition"),
		definitionSelect,
		status,
	))
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
