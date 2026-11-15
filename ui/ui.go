package ui

import (
    "fyne.io/fyne/v2/app"
)

func RunUI() {
    a := app.New()
    w := a.NewWindow("GotoSplitter")

    // Add widgets here for:
    // - Game selection dropdown
    // - Memory address input fields
    // - Status indicators (Connected/Disconnected)

    w.ShowAndRun()
}
