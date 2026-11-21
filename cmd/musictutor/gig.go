package main

import (
	"errors"

	"github.com/S95F/musicTutor/internal/career"
	"github.com/S95F/musicTutor/internal/engine"
	"github.com/S95F/musicTutor/internal/progress"
	"github.com/S95F/musicTutor/internal/scene"
	"github.com/S95F/musicTutor/internal/ui"
)

// wireGigs connects the tour map's Play gig button to performance mode.
func (o *shellOpener) wireGigs(ps *ui.PerformScreen) {
	ps.SetPlay(func(g career.Gig, crowd string, reveal bool, set []progress.MasteredSong) {
		gs := ui.NewGigScreen(o.shell, o.store, g, crowd, reveal, set, o.loadGigSong, o.CloseCurrent, o.gigCrowd)
		o.shell.Show(gs)
	})
}

// loadGigSong opens one song of a set for a gig.
func (o *shellOpener) loadGigSong(song progress.MasteredSong, recording bool) (*ui.App, error) {
	if song.Path == "" {
		return nil, errors.New("this song's file is not known any more; open it once in Practice")
	}
	app, _, err := o.open(song.Path, &gigOpen{track: song.Track})
	return app, err
}

// gigCrowd loads the crowd onto the song's engine (CAREER.md §6.4): the
// bed at −18 dB under the song, cheers and boos over it. Output only.
func (o *shellOpener) gigCrowd(*ui.App) ui.GigCrowd {
	if o.cur == nil || o.cur.eng == nil {
		return nil
	}
	eng := o.cur.eng
	eng.SetCrowd(scene.CrowdSound("bed", sampleRate))
	return &crowdSound{eng: eng, cheer: scene.CrowdSound("cheer", sampleRate), boo: scene.CrowdSound("boo", sampleRate)}
}

type crowdSound struct {
	eng        *engine.Engine
	cheer, boo []float32
}

func (c *crowdSound) SetLevel(l float64) { c.eng.SetCrowdLevel(l) }
func (c *crowdSound) Cheer()             { c.eng.PlayCrowd(c.cheer) }
func (c *crowdSound) Boo()               { c.eng.PlayCrowd(c.boo) }
