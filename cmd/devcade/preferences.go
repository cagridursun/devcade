package main

import (
	"github.com/cagridursun/devcade/internal/arcade"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/profile"
	"github.com/cagridursun/devcade/internal/terminal"
	"github.com/cagridursun/devcade/internal/ui"
	"os"
)

// Release builds may override the community service; the environment takes
// precedence, including an explicitly empty value for offline-only play.
var leaderboardURL = "https://devcade.cinesdigital.com"

func preferences() (profile.Profile, func(profile.Profile) error, string) {
	p := profile.Default()
	store, err := profile.DefaultStore()
	if err == nil {
		p, err = store.Load()
	}
	if err != nil {
		return p, nil, "Profile could not be loaded; the existing file will not be overwritten."
	}
	return p, store.Save, ""
}
func newArcade(initial string) *arcade.App {
	p, save, notice := preferences()
	endpoint := leaderboardURL
	if v, ok := os.LookupEnv("DEVCADE_LEADERBOARD_URL"); ok {
		endpoint = v
	}
	var client *leaderboard.Client
	if endpoint != "" {
		var err error
		client, err = leaderboard.NewClient(endpoint)
		if err != nil {
			notice = "Global service is not configured. Your personal bests are saved locally."
		}
	}
	return arcade.NewAppWithOptions(catalog, newDiagnostic, arcade.Options{Profile: p, Save: save, Client: client, Notice: notice, InitialGame: initial, Onboard: true, OpenURL: terminal.OpenCreatorProfile})
}

type diagnosticProgram struct {
	*engine.Engine
	language, theme string
}

func (p diagnosticProgram) Render(c engine.Canvas) {
	p.Engine.Render(ui.Canvas{Canvas: c, Language: p.language})
}
func (p diagnosticProgram) Theme() string { return p.theme }
func newConfiguredDiagnostic() terminal.Program {
	p, _, _ := preferences()
	return diagnosticProgram{Engine: engine.New(newDiagnostic()), language: p.Language, theme: p.Theme}
}
