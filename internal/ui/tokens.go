package ui

import "image/color"

// Design tokens (docs/REDESIGN.md §3). Every colour, font size and spacing
// value in internal/ui comes from here; nothing else may hold a literal.

func hex(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// The palette. Each is used for what REDESIGN §3 lists, and nothing else.
var (
	tokBG      = hex(0x111116) // window background
	tokSurface = hex(0x191920) // overlays, cards, tooltips, timeline track
	tokRaised  = hex(0x25252F) // hovered rows, pressed controls, input fields
	tokLine    = hex(0x393843) // separators, tab strings, borders
	tokText    = hex(0xF4F0E8) // primary text, fret numbers, playhead
	tokText2   = hex(0xA5A3AF) // secondary text, inactive menu items
	tokText3   = hex(0x6F6D7A) // disabled, other tracks, bar numbers
	tokBrand   = hex(0xF3A33B) // HEADLINER amber — never inside the tab area
	tokHit     = hex(0x55D69A)
	tokClose   = hex(0xE6C25E)
	tokMiss    = hex(0xE56A6A)
)

// Roles: the names the drawing code uses, each mapped onto the palette.
var (
	colBG        = tokBG
	colNoteBG    = tokBG // the knockout behind a fret number
	colPanel     = tokSurface
	colTipBG     = tokSurface
	colHover     = tokRaised
	colFocus     = tokRaised
	colOn        = tokRaised
	colOnHover   = tokLine
	colPanelEdge = tokLine
	colTipEdge   = tokLine
	colString    = tokLine
	colGtGutter  = tokLine
	colEdCursor  = tokLine

	colNote = tokText
	colHUD  = tokText

	colDim          = tokText2
	colHint         = tokText2
	colTechMark     = tokText2
	colTieArc       = tokText2
	colInferredNote = tokText2 // a note the importer inferred, on the tab

	colBarline  = tokText3
	colGroupCap = tokText3
	colDisabled = tokText3
	colEdRest   = tokText3

	// Brand amber: accents, focus, the primary action — off the tab.
	colInferred = tokBrand // titles and accents (the old accent blue)
	colAccent   = tokBrand
	colSounding = tokBrand
	colCountIn  = tokBrand
	colOnEdge   = tokBrand
	colEdCaret  = tokBrand
	colGtCaret  = tokBrand

	// The tab area: text, line and verdict colours only (§3).
	colTabSounding = tokText // the bar under a sounding note
	colPlayhead    = tokText
	colLoopEdge    = tokText
	colWaitLo      = tokText2 // a waiting note pulses text2 ↔ text
	colWaitHi      = tokText

	colHit     = tokHit
	colClose   = tokClose
	colMiss    = tokMiss
	colMissing = tokMiss
	colGtOK    = tokHit
)

// Translucent roles.
var (
	colLoop     = withAlpha(tokText, 0.08)
	colTuneZone = withAlpha(tokHit, 0.25)
	colHelpDim  = withAlpha(tokBG, 0.80)
	colScrim    = tokBG
	colWarnBG   = tokSurface
)

// Type sizes (§3). Display is the pixel font, crisp only at these sizes.
const (
	sizeUISmall = 13.0
	sizeUI      = 14.0
	sizeUILarge = 16.0
	sizeUIXL    = 20.0

	sizeMono = 14.0

	sizeDisplayS = 16.0
	sizeDisplayM = 24.0
	sizeDisplayL = 32.0
	sizeDisplayX = 48.0
)

// Spacing (§3): a 4px base.
const (
	sp1  = 4.0
	sp2  = 8.0
	sp3  = 12.0
	sp4  = 16.0
	sp6  = 24.0
	sp8  = 32.0
	sp12 = 48.0
)

// Display-font scales of the body size: the pixel font's four sizes.
const (
	dispS = sizeDisplayS / fontBody // 16
	dispM = sizeDisplayM / fontBody // 24
	dispL = sizeDisplayL / fontBody // 32
	dispX = sizeDisplayX / fontBody // 48
)

// Profile tints p1..p6 (BATTLES.md §3): one per local player, never the
// brand amber and never a verdict colour.
var profileTints = [6]color.RGBA{
	hex(0x6FA8DC), // p1 blue
	hex(0xB48EDB), // p2 violet
	hex(0x4FC1D9), // p3 cyan
	hex(0xE58FB5), // p4 pink
	hex(0x9AA6C7), // p5 slate
	hex(0xC9B79C), // p6 sand
}

// profileTint is a profile's colour.
func profileTint(i int) color.RGBA { return profileTints[((i%6)+6)%6] }
