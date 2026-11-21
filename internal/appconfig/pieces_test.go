package appconfig

import (
	"fmt"
	"path/filepath"
	"testing"
)

// songA is an absolute-looking path on any OS.
var songA = filepath.Join(string(filepath.Separator)+"songs", "a.gp")

func TestPieceSetupRoundTripsAndPrunesTheOldest(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())
	var c Config
	c.Version = CurrentVersion
	c.SetPiece(songA, PieceSetup{Track: 1, Tempo: 0.7, Pos: 3840, LoopA: 1920, LoopB: 5760,
		Muted: []bool{false, true}, Mix: &PieceMix{Master: 0.8, Tracks: []float64{1, 0}}, Used: 10})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.Piece(filepath.Dir(songA) + string(filepath.Separator) + "." + string(filepath.Separator) + "a.gp")
	if !ok || s.Track != 1 || s.Tempo != 0.7 || s.LoopB != 5760 || !s.Muted[1] || s.Mix.Tracks[1] != 0 {
		t.Fatalf("setup did not survive a save and load: %+v (found %v)", s, ok)
	}
	if s.Mix.Click != 0 {
		t.Errorf("a silent metronome came back at %v", s.Mix.Click)
	}

	for i := 0; i < MaxPieces+5; i++ {
		c.SetPiece(fmt.Sprintf("p%03d.gtab", i), PieceSetup{Used: int64(100 + i)})
	}
	if len(c.Pieces) != MaxPieces {
		t.Fatalf("kept %d setups, want the cap of %d", len(c.Pieces), MaxPieces)
	}
	if _, ok := c.Piece(songA); ok {
		t.Error("the least recently used piece survived pruning")
	}
	if _, ok := c.Piece(fmt.Sprintf("p%03d.gtab", MaxPieces+4)); !ok {
		t.Error("the newest piece was pruned")
	}
}
