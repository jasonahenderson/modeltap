package harnessshell

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// WU-107 viewport-state accessor + FEAT-0014 SC3 parity assertion.
//
// SC3: "manual scroll position is preserved when not following tail."
// Asserted by scrolling up away from the bottom, appending more
// transcript content, and confirming the viewport's YOffset did not
// auto-scroll back to bottom.
//
// The test lives in package harnessshell (not _test) because it reads
// unexported `state` fields to seed the transcript without going
// through the full submit pipeline. A future external test could use
// the [Model.ViewportState] accessor for black-box coverage.

func TestViewportStateInitialIsAtBottom(t *testing.T) {
	// A freshly constructed shell has an empty transcript; AtBottom
	// is trivially true (no content to scroll past). Width/Height
	// stay zero until WindowSizeMsg arrives.
	m := New()
	got := m.ViewportState()
	if !got.AtBottom {
		t.Fatalf("empty viewport should be AtBottom; got %+v", got)
	}
}

func TestViewportStateReflectsRenderedFrame(t *testing.T) {
	m := New()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = updated.(Model)
	got := m.ViewportState()
	if got.Width != 80 || got.Height != 12 {
		t.Fatalf("ViewportState dims = (%d,%d), want (80,12)", got.Width, got.Height)
	}
}

func TestManualScrollPreservedWhenNotFollowingTail(t *testing.T) {
	m := newWithFixedClock()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = updated.(Model)

	// Seed enough transcript content that the viewport scrolls. Each
	// item is multi-line so the rendered string greatly exceeds the
	// 8-row viewport.
	for i := 0; i < 80; i++ {
		m.state.transcriptItems = append(m.state.transcriptItems, TranscriptItem{
			ID:   "msg-" + itoa(i),
			Kind: TranscriptItemKindMessage,
			Role: RoleUser,
			Text: "seed line " + itoa(i) + " — long enough to occupy multiple rows after wrapping in a narrow viewport with the styling overhead the renderer applies",
		})
	}
	_ = m.View()
	initial := m.ViewportState()
	if !initial.AtBottom {
		t.Fatalf("seed should leave viewport at bottom; got %+v", initial)
	}

	// Scroll up via the viewport's mouse-wheel event. The bubble-tea
	// viewport handles MouseMsg{Action: MouseActionPress, Button:
	// MouseButtonWheelUp}.
	m.state.focus = FocusTranscript
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(tea.MouseMsg{
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonWheelUp,
		})
		m = updated.(Model)
	}
	afterScroll := m.ViewportState()
	if afterScroll.AtBottom {
		t.Fatalf("scrolled up but still AtBottom; got %+v", afterScroll)
	}

	// Append a streaming-style delta via a host event so refresh
	// fires. SC3: the YOffset must not flip back to bottom.
	updated, _ = m.Update(HostStatusEvent{
		Status: "newly-streamed status",
		Kind:   StatusStreaming,
	})
	m = updated.(Model)
	afterAppend := m.ViewportState()
	if afterAppend.AtBottom {
		t.Fatalf("manual scroll lost on append (auto-followed tail): before=%+v after=%+v",
			afterScroll, afterAppend)
	}
	if afterAppend.YOffset != afterScroll.YOffset {
		t.Fatalf("YOffset moved on append: before=%d after=%d (SC3 says scroll preserved)",
			afterScroll.YOffset, afterAppend.YOffset)
	}
}

// PATCH-0034: PgUp / PgDn and Alt+Up / Alt+Down must scroll the
// transcript regardless of which focus zone owns the keyboard. The
// default (input-focused, no /select) state needs a discoverable
// keyboard scroll path after PATCH-0030 turned mouse capture off.
func TestFocusAgnosticScrollFromInputFocus(t *testing.T) {
	m := newWithFixedClock()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = updated.(Model)

	for i := 0; i < 80; i++ {
		m.state.transcriptItems = append(m.state.transcriptItems, TranscriptItem{
			ID:   "msg-" + itoa(i),
			Kind: TranscriptItemKindMessage,
			Role: RoleUser,
			Text: "seed line " + itoa(i) + " — long enough to occupy multiple rows after wrapping in a narrow viewport with the styling overhead the renderer applies",
		})
	}
	// Prime the viewport content via Update so refresh() pushes the
	// seeded transcriptItems into the underlying viewport. Without
	// this, the first scroll event scrolls an empty viewport (no-op)
	// while refresh() then fills it.
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = updated.(Model)

	if !m.state.input.Focused() {
		t.Fatalf("precondition: input should be focused by default")
	}
	if m.state.focus != FocusInput {
		t.Fatalf("precondition: focus = %v, want FocusInput", m.state.focus)
	}
	if !m.ViewportState().AtBottom {
		t.Fatalf("seed should leave viewport at bottom")
	}

	// PgUp from input focus scrolls the transcript without changing focus.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	if m.state.focus != FocusInput {
		t.Errorf("PgUp changed focus: %v", m.state.focus)
	}
	if m.ViewportState().AtBottom {
		t.Errorf("PgUp from input focus did not scroll: still AtBottom")
	}

	afterPgUp := m.ViewportState().YOffset

	// Alt+Up nudges by one line, still from input focus.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp, Alt: true})
	m = updated.(Model)
	afterAltUp := m.ViewportState().YOffset
	if afterAltUp >= afterPgUp {
		t.Errorf("Alt+Up did not move YOffset up: before=%d after=%d", afterPgUp, afterAltUp)
	}

	// PgDn scrolls back down.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)
	afterPgDn := m.ViewportState().YOffset
	if afterPgDn <= afterAltUp {
		t.Errorf("PgDn did not move YOffset down: before=%d after=%d", afterAltUp, afterPgDn)
	}

	// Alt+Down also nudges.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: true})
	m = updated.(Model)
	afterAltDn := m.ViewportState().YOffset
	if afterAltDn <= afterPgDn {
		t.Errorf("Alt+Down did not move YOffset down: before=%d after=%d", afterPgDn, afterAltDn)
	}

	// Plain Up arrow with empty input still recalls history (or is a
	// no-op when history is empty); it must NOT scroll the viewport.
	beforePlain := m.ViewportState().YOffset
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if got := m.ViewportState().YOffset; got != beforePlain {
		t.Errorf("plain Up changed viewport YOffset (should be history recall instead): before=%d after=%d", beforePlain, got)
	}
}

// itoa is a small int-to-string helper so the test does not import
// strconv just for transcript seeding.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf strings.Builder
	if n < 0 {
		buf.WriteByte('-')
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	buf.Write(digits)
	return buf.String()
}
