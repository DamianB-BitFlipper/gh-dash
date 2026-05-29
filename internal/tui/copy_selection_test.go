package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dlvhdr/gh-dehub/v4/internal/tui/components/selection"
)

func TestCopySelectionScopesToInnermostRegionAndClamps(t *testing.T) {
	selection.Reset()
	t.Cleanup(selection.Reset)

	outer := selection.ID("test-outer")
	inner := selection.ID("test-inner")

	// A coarse pane region and a nested sub-component region (as produced by
	// computed geometry).
	selection.RegisterBounds(outer, selection.Bounds{X: 0, Y: 0, Width: 40, Height: 20},
		"big pane", "big pane")
	selection.RegisterBounds(inner, selection.Bounds{X: 2, Y: 3, Width: 9, Height: 1},
		"INNERTEXT", "INNERTEXT")

	ib, _, _, ok := selection.Region(inner)
	if !ok {
		t.Fatalf("expected inner region to resolve")
	}

	// A click inside the inner region must scope selection to the inner region.
	regionID, bounds := copySelectionRegionAt(ib.X, ib.Y)
	if regionID != inner {
		t.Fatalf("expected innermost region %q, got %q", inner, regionID)
	}

	var m Model
	m.copySelection.begin(regionID, ib.X, ib.Y)
	// Drag far past the inner region; clamping must keep the end inside it.
	clampedX, clampedY := clampCopySelectionPoint(ib.X+1000, ib.Y+1000, bounds)
	m.copySelection.update(clampedX, clampedY)

	got := m.copySelectionText()
	if got != "INNERTEXT" {
		t.Fatalf("expected selection clamped to inner region text %q, got %q", "INNERTEXT", got)
	}
}

// Regression test for the highlight blowing out formatting: the highlight is
// applied to STYLED content so the unselected prefix/suffix keep their ANSI
// styling and only the selected span is recolored.
func TestRenderCopySelectionHighlightPreservesStyledContent(t *testing.T) {
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("8"))
	styled := textStyle.Render("hello world")

	bounds := selection.Bounds{X: 0, Y: 0, Width: 11, Height: 1}
	// Select only "world" (cols 6-10).
	got := renderCopySelectionHighlight(styled, bounds, 6, 0, 10, 0, highlight)

	if ansi.Strip(got) != "hello world" {
		t.Fatalf("expected visible text preserved, got %q", ansi.Strip(got))
	}
	// The unselected prefix must retain the original foreground styling.
	prefix := ansi.Cut(got, 0, 5)
	if !strings.Contains(prefix, "\x1b[") {
		t.Fatalf("expected styled prefix to retain ANSI styling, got %q", prefix)
	}
}

func TestRegisterScrollComputesVisibleBounds(t *testing.T) {
	selection.Reset()
	t.Cleanup(selection.Reset)

	a := selection.ID("scroll-a")
	b := selection.ID("scroll-b")
	c := selection.ID("scroll-c")

	// Three stacked blocks of height 2 each, content origin (5, 10), width 30,
	// 4 visible rows, scrolled down by 1 line.
	registerScroll(selection.Scroll{
		OriginX:       5,
		OriginY:       10,
		Width:         30,
		VisibleHeight: 4,
		YOffset:       1,
		Blocks: []selection.Block{
			{ID: a, ContentY: 0, Height: 2, Plain: "a0\na1", Styled: "a0\na1"},
			{ID: b, ContentY: 2, Height: 2, Plain: "b0\nb1", Styled: "b0\nb1"},
			{ID: c, ContentY: 4, Height: 2, Plain: "c0\nc1", Styled: "c0\nc1"},
		},
	})

	// Block a: contentY 0..2, scrolled by 1 -> only its second line (a1) visible
	// at screen y = 10. Leading line dropped.
	ba, pa, _, ok := selection.Region(a)
	if !ok {
		t.Fatalf("expected block a to resolve")
	}
	if ba.Y != 10 || ba.Height != 1 || ba.X != 5 || ba.Width != 30 {
		t.Fatalf("unexpected block a bounds: %+v", ba)
	}
	if pa != "a1" {
		t.Fatalf("expected clipped content %q, got %q", "a1", pa)
	}

	// Block b: fully visible at screen y = 11..13.
	bb, _, _, ok := selection.Region(b)
	if !ok || bb.Y != 11 || bb.Height != 2 {
		t.Fatalf("unexpected block b: bounds=%+v ok=%v", bb, ok)
	}

	// Block c: starts at contentY 4; with offset 1 and 4 visible rows, the
	// window covers content lines 1..4 (exclusive 5), so only c's first line is
	// visible at screen y = 13.
	bc, pc, _, ok := selection.Region(c)
	if !ok || bc.Y != 13 || bc.Height != 1 || pc != "c0" {
		t.Fatalf("unexpected block c: bounds=%+v plain=%q ok=%v", bc, pc, ok)
	}
}

func TestExtractCopySelectionTextClampsToRegionContent(t *testing.T) {
	bounds := selection.Bounds{X: 10, Y: 5, Width: 8, Height: 3}
	content := "alpha bravo\ncharlie delta\necho foxtrot"

	got := extractCopySelectionText(content, bounds, 12, 5, 25, 6)
	want := "pha br\ncharlie"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExtractCopySelectionTextHandlesReverseDrag(t *testing.T) {
	bounds := selection.Bounds{X: 0, Y: 0, Width: 10, Height: 2}
	content := "0123456789\nabcdefghij"

	got := extractCopySelectionText(content, bounds, 6, 1, 2, 0)
	want := "23456789\nabcdefg"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExtractCopySelectionTextStripsANSI(t *testing.T) {
	bounds := selection.Bounds{X: 0, Y: 0, Width: 10, Height: 1}
	content := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("copy me")

	got := extractCopySelectionText(content, bounds, 0, 0, 6, 0)
	want := "copy me"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestClampCopySelectionPointStaysInsideBounds(t *testing.T) {
	bounds := selection.Bounds{X: 5, Y: 3, Width: 10, Height: 4}

	x, y := clampCopySelectionPoint(100, -1, bounds)
	if x != 14 || y != 3 {
		t.Fatalf("expected clamped point (14, 3), got (%d, %d)", x, y)
	}
}

func TestRenderCopySelectionHighlightDoesNotLeakRawANSI(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("8"))
	content := style.Render("copy me")

	got := renderCopySelectionHighlight(content, selection.Bounds{X: 0, Y: 0, Width: 10, Height: 1}, 0, 0, 3, 0, highlight)
	if strings.Contains(ansi.Strip(got), "[38;2;") {
		t.Fatalf("highlight leaked raw ANSI text: %q", got)
	}
	if ansi.Strip(got) != "copy me" {
		t.Fatalf("expected visible text to remain unchanged, got %q", ansi.Strip(got))
	}
}

func TestRenderCopySelectionHighlightPreservesUnselectedStyling(t *testing.T) {
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	highlight := lipgloss.NewStyle().Background(lipgloss.Color("8"))
	content := textStyle.Render("copy me")

	got := renderCopySelectionHighlight(content, selection.Bounds{X: 0, Y: 0, Width: 10, Height: 1}, 2, 0, 3, 0, highlight)
	prefix := ansi.Cut(got, 0, 2)
	suffix := ansi.Cut(got, 4, lipgloss.Width(got))

	if !strings.Contains(prefix, "\x1b[") {
		t.Fatalf("expected styled prefix to retain ANSI styling, got %q", prefix)
	}
	if !strings.Contains(suffix, "\x1b[") {
		t.Fatalf("expected styled suffix to retain ANSI styling, got %q", suffix)
	}
}

func TestClipCopySelectionContentClipsLongLines(t *testing.T) {
	got := clipCopySelectionContent("0123456789", selection.Bounds{Width: 5, Height: 1})
	want := "01234"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if width := lipgloss.Width(got); width != 5 {
		t.Fatalf("expected width 5, got %d", width)
	}
}

func TestClipCopySelectionContentClipsExtraLines(t *testing.T) {
	got := clipCopySelectionContent("one\ntwo\nthree", selection.Bounds{Width: 10, Height: 2})
	want := "one\ntwo"

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestClipCopySelectionContentPreservesANSISafely(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	got := clipCopySelectionContent(style.Render("0123456789"), selection.Bounds{Width: 5, Height: 1})

	if ansi.Strip(got) != "01234" {
		t.Fatalf("expected visible text to be clipped, got %q", ansi.Strip(got))
	}
	if strings.Contains(ansi.Strip(got), "[38;") {
		t.Fatalf("clip leaked raw ANSI text: %q", got)
	}
}
