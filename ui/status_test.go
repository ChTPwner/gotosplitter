package ui

import (
	"image/color"
	"os"
	"sync"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestMain(m *testing.M) {
	test.NewApp()
	os.Exit(m.Run())
}

func TestResolveStatusColorAndState(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantState StatusState
	}{
		// SNI states
		{name: "SNI Disconnected bare", input: "Disconnected", wantState: StatusStateDisconnected},
		{name: "SNI Disconnected retrying", input: "Disconnected (Retrying...)", wantState: StatusStateDisconnected},
		{name: "SNI Connecting bare", input: "Connecting...", wantState: StatusStateConnecting},
		{name: "SNI Connecting to url", input: "Connecting to ws://localhost:8080...", wantState: StatusStateConnecting},
		{name: "SNI Attached device", input: "Attached: RetroArch", wantState: StatusStateConnected},
		{name: "SNI Attached to hardware", input: "Attached to: SD2SNES COM3", wantState: StatusStateConnected},

		// LiveSplit states
		{name: "LiveSplit Waiting default", input: "Waiting for LiveSplit One...", wantState: StatusStateConnecting},
		{name: "LiveSplit Waiting client", input: "Waiting for LiveSplit One client...", wantState: StatusStateConnecting},
		{name: "LiveSplit Connected 1 client", input: "Connected (1 client)", wantState: StatusStateConnected},
		{name: "LiveSplit Connected multiple", input: "Connected (2 clients)", wantState: StatusStateConnected},
		{name: "LiveSplit Connected with url", input: "Connected (ws://localhost:16834)", wantState: StatusStateConnected},
		{name: "LiveSplit Stopped", input: "Stopped", wantState: StatusStateDisconnected},

		// Edge cases
		{name: "Empty string", input: "", wantState: StatusStateDisconnected},
		{name: "Whitespace only", input: "   ", wantState: StatusStateDisconnected},
		{name: "Error message", input: "Error: dial tcp refused", wantState: StatusStateDisconnected},
		{name: "Failed message", input: "Failed to connect", wantState: StatusStateDisconnected},
		{name: "Mixed case", input: "attached: snes9x", wantState: StatusStateConnected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col, state := ResolveStatusColorAndState(tt.input)
			if state != tt.wantState {
				t.Fatalf("ResolveStatusColorAndState(%q) state = %v, want %v", tt.input, state, tt.wantState)
			}
			wantCol := ColorForState(tt.wantState)
			if col != wantCol {
				t.Fatalf("ResolveStatusColorAndState(%q) color = %v, want %v", tt.input, col, wantCol)
			}
		})
	}
}

func TestColorForStatusText(t *testing.T) {
	col := ColorForStatusText("Attached: RetroArch")
	if col != ColorForState(StatusStateConnected) {
		t.Fatalf("ColorForStatusText() = %v, want %v", col, ColorForState(StatusStateConnected))
	}
}

func TestStatusStateString(t *testing.T) {
	if got := StatusStateConnected.String(); got != "Connected" {
		t.Fatalf("StatusStateConnected.String() = %q, want Connected", got)
	}
	if got := StatusStateConnecting.String(); got != "Connecting" {
		t.Fatalf("StatusStateConnecting.String() = %q, want Connecting", got)
	}
	if got := StatusStateDisconnected.String(); got != "Disconnected" {
		t.Fatalf("StatusStateDisconnected.String() = %q, want Disconnected", got)
	}
	if got := StatusState(99).String(); got != "Unknown" {
		t.Fatalf("StatusState(99).String() = %q, want Unknown", got)
	}
}

func TestStatusIndicatorLifecycle(t *testing.T) {
	indicator := NewStatusIndicator("Disconnected", nil)
	if indicator == nil {
		t.Fatal("NewStatusIndicator returned nil")
	}

	if got := indicator.Text(); got != "Disconnected" {
		t.Fatalf("indicator.Text() = %q, want Disconnected", got)
	}
	if got := indicator.State(); got != StatusStateDisconnected {
		t.Fatalf("indicator.State() = %v, want %v", got, StatusStateDisconnected)
	}
	if got := indicator.Color(); got != ColorForState(StatusStateDisconnected) {
		t.Fatalf("indicator.Color() = %v, want %v", got, ColorForState(StatusStateDisconnected))
	}
	if indicator.Circle() == nil {
		t.Fatal("indicator.Circle() returned nil")
	}
	if indicator.Label() == nil {
		t.Fatal("indicator.Label() returned nil")
	}

	// Update to Connecting
	indicator.SetStatus("Connecting to ws://localhost:8080...")
	if got := indicator.Text(); got != "Connecting to ws://localhost:8080..." {
		t.Fatalf("indicator.Text() = %q", got)
	}
	if got := indicator.State(); got != StatusStateConnecting {
		t.Fatalf("indicator.State() = %v, want %v", got, StatusStateConnecting)
	}
	if got := indicator.Color(); got != ColorForState(StatusStateConnecting) {
		t.Fatalf("indicator.Color() = %v, want %v", got, ColorForState(StatusStateConnecting))
	}

	// Update to Attached
	indicator.SetStatus("Attached: RetroArch")
	if got := indicator.Text(); got != "Attached: RetroArch" {
		t.Fatalf("indicator.Text() = %q", got)
	}
	if got := indicator.State(); got != StatusStateConnected {
		t.Fatalf("indicator.State() = %v, want %v", got, StatusStateConnected)
	}
	if got := indicator.Color(); got != ColorForState(StatusStateConnected) {
		t.Fatalf("indicator.Color() = %v, want %v", got, ColorForState(StatusStateConnected))
	}

	// Update with custom color
	customColor := color.NRGBA{R: 100, G: 100, B: 200, A: 255}
	indicator.SetStatusWithColor("Custom Status", customColor)
	if got := indicator.Text(); got != "Custom Status" {
		t.Fatalf("indicator.Text() = %q", got)
	}
	if got := indicator.Color(); got != customColor {
		t.Fatalf("indicator.Color() = %v, want %v", got, customColor)
	}

	// Update via SetState
	indicator.SetState(StatusStateDisconnected, "Disconnected (Retrying...)")
	if got := indicator.State(); got != StatusStateDisconnected {
		t.Fatalf("indicator.State() = %v, want %v", got, StatusStateDisconnected)
	}
	if got := indicator.Text(); got != "Disconnected (Retrying...)" {
		t.Fatalf("indicator.Text() = %q", got)
	}
}

func TestStatusIndicatorConcurrency(t *testing.T) {
	indicator := NewStatusIndicator("Initial", nil)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				indicator.SetStatus("Attached: RetroArch")
			} else {
				indicator.SetStatus("Disconnected (Retrying...)")
			}
		}(i)
		go func() {
			defer wg.Done()
			_ = indicator.Text()
			_ = indicator.Color()
			_ = indicator.State()
		}()
	}
	wg.Wait()
}

func TestStatusIndicatorWidgetRendering(t *testing.T) {
	indicator := NewStatusIndicator("Attached: RetroArch", nil)

	// Verify indicator satisfies fyne.Widget interface
	var _ fyne.Widget = indicator

	renderer := test.WidgetRenderer(indicator)
	if renderer == nil {
		t.Fatal("expected non-nil renderer for StatusIndicator")
	}

	objects := renderer.Objects()
	if len(objects) == 0 {
		t.Fatal("expected renderer to have child objects")
	}

	if indicator.Container() == nil {
		t.Fatal("expected indicator.Container() to be non-nil")
	}

	// Verify minimum size is non-empty (circle + label)
	minSize := indicator.MinSize()
	if minSize.Width <= 0 || minSize.Height <= 0 {
		t.Fatalf("expected positive MinSize, got %v", minSize)
	}

	// Verify text and circle are in the container
	if indicator.Label().Text != "Attached: RetroArch" {
		t.Fatalf("label text = %q, want Attached: RetroArch", indicator.Label().Text)
	}
	if indicator.Circle().FillColor != ColorForState(StatusStateConnected) {
		t.Fatalf("circle color = %v, want green", indicator.Circle().FillColor)
	}
}
