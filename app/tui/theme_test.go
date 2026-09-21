package tui

import "testing"

func TestWarmCopperPaletteProfiles(t *testing.T) {
	trueColour := paletteFor(ColorTrue)
	if trueColour.Canvas != "#0B0F10" || trueColour.Copper != "#D39463" || trueColour.DataBlue != "#8EC8E8" {
		t.Fatalf("true colour palette drifted: %+v", trueColour)
	}
	ansi := paletteFor(ColorANSI256)
	if ansi.Copper != "173" || ansi.Error != "203" {
		t.Fatalf("256-colour mapping drifted: %+v", ansi)
	}
	none := paletteFor(ColorNone)
	if none.Text != "" || none.Copper != "" || none.Error != "" {
		t.Fatalf("NO_COLOR palette contains colours: %+v", none)
	}
}

func TestResponsiveBreakpoints(t *testing.T) {
	if chooseLayout(120, 30) != layoutWide || chooseLayout(80, 24) != layoutStandard || chooseLayout(32, 12) != layoutCompact {
		t.Fatal("responsive breakpoint contract changed")
	}
}
