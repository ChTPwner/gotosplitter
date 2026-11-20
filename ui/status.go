package ui

import (
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Standard status indicator colors.
var (
	// StatusColorRed represents the Disconnected or Error state color (#dc3545).
	StatusColorRed = color.NRGBA{R: 220, G: 53, B: 69, A: 255}
	// StatusColorYellow represents the Connecting or Waiting state color (#ffc107).
	StatusColorYellow = color.NRGBA{R: 255, G: 193, B: 7, A: 255}
	// StatusColorGreen represents the Attached or Connected state color (#28a745).
	StatusColorGreen = color.NRGBA{R: 40, G: 167, B: 69, A: 255}
)

// StatusState represents the high-level connection state for status indicators.
type StatusState int

const (
	StatusStateDisconnected StatusState = iota
	StatusStateConnecting
	StatusStateConnected
)

func (s StatusState) String() string {
	switch s {
	case StatusStateConnected:
		return "Connected"
	case StatusStateConnecting:
		return "Connecting"
	case StatusStateDisconnected:
		return "Disconnected"
	default:
		return "Unknown"
	}
}

// ColorForState returns the color associated with a StatusState.
func ColorForState(state StatusState) color.Color {
	switch state {
	case StatusStateConnected:
		return StatusColorGreen
	case StatusStateConnecting:
		return StatusColorYellow
	case StatusStateDisconnected:
		fallthrough
	default:
		return StatusColorRed
	}
}

// ResolveStatusColorAndState inspects status text and returns its corresponding color and StatusState.
func ResolveStatusColorAndState(statusText string) (color.Color, StatusState) {
	trimmed := strings.TrimSpace(statusText)
	lower := strings.ToLower(trimmed)
	if lower == "" {
		return ColorForState(StatusStateDisconnected), StatusStateDisconnected
	}

	// Attached or Connected states -> Green
	if strings.Contains(lower, "attached") || strings.HasPrefix(lower, "connected") {
		return ColorForState(StatusStateConnected), StatusStateConnected
	}

	// Connecting, Waiting, or Retrying -> Yellow
	if strings.Contains(lower, "connecting") || strings.Contains(lower, "waiting") {
		return ColorForState(StatusStateConnecting), StatusStateConnecting
	}

	// Disconnected, Error, Failed, or Stopped -> Red
	if strings.Contains(lower, "disconnected") || strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "stopped") {
		return ColorForState(StatusStateDisconnected), StatusStateDisconnected
	}

	// Default fallback: Red (Disconnected)
	return ColorForState(StatusStateDisconnected), StatusStateDisconnected
}

// ColorForStatusText returns the color associated with a status text string.
func ColorForStatusText(statusText string) color.Color {
	col, _ := ResolveStatusColorAndState(statusText)
	return col
}

// StatusIndicator is a Fyne UI widget combining a colored badge/circle indicator with a text label.
type StatusIndicator struct {
	widget.BaseWidget
	circle    *canvas.Circle
	label     *widget.Label
	container *fyne.Container
	mu        sync.RWMutex
	state     StatusState
	text      string
	color     color.Color
}

// NewStatusIndicator creates a new status indicator with the given initial text and color.
func NewStatusIndicator(initialText string, initialColor color.Color) *StatusIndicator {
	if initialColor == nil {
		initialColor, _ = ResolveStatusColorAndState(initialText)
	}
	_, state := ResolveStatusColorAndState(initialText)

	circle := canvas.NewCircle(initialColor)
	circleWrap := container.NewGridWrap(fyne.NewSize(12, 12), circle)
	centeredCircle := container.NewCenter(circleWrap)
	label := widget.NewLabel(initialText)
	box := container.NewHBox(centeredCircle, label)

	ind := &StatusIndicator{
		circle:    circle,
		label:     label,
		container: box,
		state:     state,
		text:      initialText,
		color:     initialColor,
	}
	ind.ExtendBaseWidget(ind)
	return ind
}

// CreateRenderer implements fyne.Widget by providing a renderer for the contained objects.
func (s *StatusIndicator) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.container)
}

// SetStatus updates the indicator text and automatically determines the color and state.
func (s *StatusIndicator) SetStatus(text string) {
	col, state := ResolveStatusColorAndState(text)
	s.SetStateAndColor(state, text, col)
}

// SetStatusWithColor updates the indicator text with an explicit color.
func (s *StatusIndicator) SetStatusWithColor(text string, col color.Color) {
	_, state := ResolveStatusColorAndState(text)
	s.SetStateAndColor(state, text, col)
}

// SetState updates the indicator state, automatically resolving the color for that state.
func (s *StatusIndicator) SetState(state StatusState, text string) {
	s.SetStateAndColor(state, text, ColorForState(state))
}

// SetStateAndColor updates the state, text, and color, and safely refreshes the canvas objects.
func (s *StatusIndicator) SetStateAndColor(state StatusState, text string, col color.Color) {
	s.mu.Lock()
	s.state = state
	s.text = text
	s.color = col
	s.mu.Unlock()

	updateUI := func() {
		s.circle.FillColor = col
		s.circle.Refresh()
		s.label.SetText(text)
		s.Refresh()
	}

	if fyne.CurrentApp() == nil {
		updateUI()
	} else {
		fyne.Do(updateUI)
	}
}

// Text returns the current status text.
func (s *StatusIndicator) Text() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.text
}

// Color returns the current status color.
func (s *StatusIndicator) Color() color.Color {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.color
}

// State returns the current status state.
func (s *StatusIndicator) State() StatusState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Circle returns the underlying canvas.Circle object for direct inspection.
func (s *StatusIndicator) Circle() *canvas.Circle {
	return s.circle
}

// Label returns the underlying widget.Label object for direct inspection.
func (s *StatusIndicator) Label() *widget.Label {
	return s.label
}

// Container returns the underlying fyne.Container layout.
func (s *StatusIndicator) Container() *fyne.Container {
	return s.container
}
