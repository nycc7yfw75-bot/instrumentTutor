package ui

import ()

type onboardStep struct {
	title  string
	detail string
	done   bool

	act func()
}

func (b *Browser) stepList() []onboardStep {
	var svc Services
	if b.sh != nil {
		svc = b.sh.Services()
	}
	var capID, playID string
	if svc.Prefs != nil {
		capID, playID = svc.Prefs.Devices()
	}

	var steps []onboardStep
	if svc.Audio == nil {

		steps = append(steps, onboardStep{
			title:  "Playback only on this machine",
			detail: "no audio capture backend, so the app cannot score your playing",
		})
	} else {
		detail := "where your guitar or mic plugs in — click here or press S"
		if capID != "" {
			detail = "chosen; change it in settings whenever you move interfaces"
		}
		steps = append(steps, onboardStep{
			title:  "Choose your audio interface",
			detail: detail,
			done:   capID != "",
			act:    b.settings,
		})

		calibrated := false
		if capID != "" && playID != "" {
			_, calibrated = svc.Audio.CalibratedOffset(capID, playID)
		}
		steps = append(steps, onboardStep{
			title:  "Calibrate the timing",
			detail: "one pass of clicks that measures your setup's delay",
			done:   calibrated,
			act:    b.settings,
		})
	}
	steps = append(steps, onboardStep{
		title:  "Open a piece, or write one",
		detail: "import a file (O), or start from a blank staff (N)",
		done:   b.hasAnyPiece(),
		act:    func() { b.launchOpenDialog("") },
	})
	return steps
}

func (b *Browser) hasAnyPiece() bool {
	for _, p := range b.paneOrder() {
		if len(b.panes[p]) > 0 {
			return true
		}
	}
	return false
}

func (b *Browser) activateStep(i int) {
	steps := b.stepList()
	if i < 0 || i >= len(steps) {
		return
	}
	if act := steps[i].act; act != nil {
		act()
	}
}

func (b *Browser) toggleHint() {
	b.hintOpen = !b.hintOpen
	if pr := b.prefs(); pr != nil {
		pr.SetHintHidden(!b.hintOpen)

		_ = pr.Save()
	}
}
