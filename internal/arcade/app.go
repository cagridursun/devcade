package arcade

import (
	"context"
	"fmt"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/metrics"
	"github.com/cagridursun/devcade/internal/profile"
	"github.com/cagridursun/devcade/internal/ui"
	"sync"
	"time"
)

const (
	MenuWidth  = 80
	MenuHeight = 24
)
const menuFooterPlay = " Up/Down: select  Enter: open  O: settings  D: diagnostic  Q / Esc: quit"
const menuFooterInfo = menuFooterPlay
const activityFooter = " Q / Esc: back to game menu   Ctrl+C: quit DevCade"
const TwitterURL = "https://x.com/c__dursun"

type screen uint8

const (
	mainMenu screen = iota
	gameMenu
	settings
	username
	scores
)

type Options struct {
	Profile     profile.Profile
	Save        func(profile.Profile) error
	Client      *leaderboard.Client
	Metrics     *metrics.Client
	Onboard     bool
	InitialGame string
	Notice      string
	OpenURL     func(context.Context, string) error
}
type result struct {
	game, name   string
	board        leaderboard.Board
	registration *leaderboard.Registration
	err          error
}

// The terminal goroutine owns App. Workers only receive immutable snapshots.
type App struct {
	catalog                 Catalog
	diagnostic              func() engine.Game
	width, height, selected int
	notice                  string
	active                  *engine.Engine
	game                    engine.Game
	state                   screen
	activity                string
	choice, setting, scroll int
	profile                 profile.Profile
	save                    func(profile.Profile) error
	client                  *leaderboard.Client
	metrics                 *metrics.Client
	ctx                     context.Context
	cancel                  context.CancelFunc
	wg                      sync.WaitGroup
	results                 chan result
	busy, pending, opening  bool
	boards                  map[string]leaderboard.Board
	networkNotice           string
	nameInput               string
	returnTo                screen
	shareAfterName          bool
	openURL                 func(context.Context, string) error
	social                  chan error
}

func NewApp(c Catalog, d func() engine.Game) *App {
	return NewAppWithOptions(c, d, Options{Profile: profile.Default()})
}
func NewAppWithOptions(c Catalog, d func() engine.Game, o Options) *App {
	ctx, cancel := context.WithCancel(context.Background())
	p := o.Profile
	if p.Language == "" {
		p = profile.Default()
	}
	p = p.Clone()
	a := &App{catalog: c, diagnostic: d, profile: p, save: o.Save, client: o.Client, metrics: o.Metrics, notice: o.Notice, ctx: ctx, cancel: cancel, results: make(chan result, 1), social: make(chan error, 1), boards: map[string]leaderboard.Board{}, openURL: o.OpenURL}
	a.configureMetrics()
	if o.InitialGame != "" {
		for i := range c.Len() {
			if c.Entry(i).ID == o.InitialGame {
				a.selected = i
				a.state = gameMenu
			}
		}
	}
	if o.Onboard && p.Username == "" {
		a.returnTo = a.state
		a.state = username
	}
	return a
}
func (a *App) Close() {
	a.endMetrics("closed")
	a.cancel()
	a.wg.Wait()
	a.metrics.Close()
}
func (a *App) Theme() string   { return a.profile.Theme }
func (a *App) menuReady() bool { return a.width >= MenuWidth && a.height >= MenuHeight }
func (a *App) Resize(w, h int) {
	a.width, a.height = max(w, 0), max(h, 0)
	if a.active != nil {
		a.active.Resize(a.width, a.height)
	}
}
func (a *App) persist() bool {
	if a.save != nil {
		if err := a.save(a.profile.Clone()); err != nil {
			a.notice = "Profile could not be saved. Changes apply only to this session."
			return false
		}
	}
	return true
}
func (a *App) observeScore() {
	f, ok := a.game.(engine.Finisher)
	if !ok || !f.Finished() {
		return
	}
	g, ok := a.game.(interface{ Score() int })
	if !ok || !profile.ValidGame(a.activity) {
		return
	}
	if a.profile.Record(a.activity, g.Score()) {
		a.persist()
		if a.profile.Share {
			a.sync(a.activity)
		}
	}
}
func (a *App) Input(ev engine.Event) bool {
	a.poll()
	if ev.Key == engine.KeyExit {
		a.endMetrics("closed")
		a.observeScore()
		return true
	}
	if a.active != nil {
		if ev.Key == engine.KeyBack {
			a.endMetrics("left")
			a.observeScore()
			a.active = nil
			a.game = nil
			if a.activity == "diagnostic" {
				a.state = mainMenu
			} else {
				a.state = gameMenu
			}
			a.choice = 0
			return false
		}
		a.active.Input(ev)
		a.observeScore()
		return false
	}
	if a.state == username {
		return a.nameEvent(ev)
	}
	if ev.Key == engine.KeyBack {
		switch a.state {
		case mainMenu:
			return true
		case scores:
			a.state = gameMenu
		case gameMenu, settings:
			a.state = mainMenu
		}
		a.notice = ""
		return false
	}
	if !a.menuReady() {
		return false
	}
	switch a.state {
	case mainMenu:
		if ev.Char == 'd' {
			a.activity = "diagnostic"
			a.launch(a.diagnostic)
			return false
		}
		if ev.Char == 'o' {
			a.state = settings
			a.setting = 0
			return false
		}
		n := a.catalog.Len() + 2
		switch ev.Key {
		case engine.KeyUp:
			a.selected = (a.selected + n - 1) % n
			a.notice = ""
		case engine.KeyDown:
			a.selected = (a.selected + 1) % n
			a.notice = ""
		case engine.KeySelect:
			if a.selected == a.catalog.Len() {
				a.state = settings
				a.setting = 0
			} else if a.selected == a.catalog.Len()+1 {
				a.openCreator()
			} else if e := a.entry(); e.Available() {
				a.state = gameMenu
				a.choice = 0
				a.notice = ""
			} else {
				a.notice = fmt.Sprintf("%s is not playable yet: it is planned for %s.", e.Name, e.Milestone)
			}
		}
	case gameMenu:
		if ev.Key == engine.KeyUp || ev.Key == engine.KeyDown {
			a.choice = 1 - a.choice
		}
		if ev.Key == engine.KeySelect {
			if a.choice == 0 {
				a.activity = a.entry().ID
				a.launch(a.entry().New)
			} else {
				a.state = scores
				a.scroll = 0
				a.sync(a.entry().ID)
			}
		}
	case settings:
		a.settingsInput(ev)
	case scores:
		if ev.Char == 'r' {
			if a.profile.Share {
				a.sync(a.entry().ID)
			}
		}
		if ev.Key == engine.KeyDown {
			a.scroll = min(a.scroll+1, max(0, len(a.boards[a.entry().ID].Rows)-12))
		}
		if ev.Key == engine.KeyUp {
			a.scroll = max(0, a.scroll-1)
		}
	}
	return false
}
func (a *App) entry() Entry { return a.catalog.Entry(min(a.selected, a.catalog.Len()-1)) }
func (a *App) nameEvent(ev engine.Event) bool {
	if ev.Key == engine.KeyBack && ev.Char == 0 {
		a.nameInput = ""
		a.shareAfterName = false
		a.state = a.returnTo
		return false
	}
	if !a.menuReady() {
		return false
	}
	switch {
	case ev.Char > 0:
		if len(a.nameInput) < 20 && ((ev.Char >= 'a' && ev.Char <= 'z') || (ev.Char >= '0' && ev.Char <= '9') || ev.Char == '_') {
			a.nameInput += string(ev.Char)
		}
	case ev.Key == engine.KeyErase:
		if len(a.nameInput) > 0 {
			a.nameInput = a.nameInput[:len(a.nameInput)-1]
		}
	case ev.Key == engine.KeySelect:
		if profile.ValidUsername(a.nameInput) {
			if a.profile.Username != a.nameInput {
				a.profile.Identity = profile.Identity{}
			}
			a.profile.Username = a.nameInput
			if a.shareAfterName {
				a.profile.Share = true
			}
			a.shareAfterName = false
			a.state = a.returnTo
			a.notice = ""
			a.persist()
			if a.profile.Share {
				a.sync(a.entry().ID)
			}
		}
	}
	return false
}
func (a *App) settingsInput(ev engine.Event) {
	if ev.Key == engine.KeyUp {
		a.setting = (a.setting + 4) % 5
		return
	}
	if ev.Key == engine.KeyDown {
		a.setting = (a.setting + 1) % 5
		return
	}
	if ev.Key != engine.KeyLeft && ev.Key != engine.KeyRight && ev.Key != engine.KeySelect {
		return
	}
	step := 1
	if ev.Key == engine.KeyLeft {
		step = -1
	}
	cycle := func(value string, list []string) string {
		index := 0
		for i, s := range list {
			if s == value {
				index = i
			}
		}
		return list[(index+step+len(list))%len(list)]
	}
	a.notice = ""
	switch a.setting {
	case 0:
		a.profile.Language = cycle(a.profile.Language, ui.Languages)
	case 1:
		a.profile.Theme = cycle(a.profile.Theme, ui.Themes)
	case 2:
		a.returnTo = settings
		a.state = username
		a.nameInput = a.profile.Username
		return
	case 3:
		if a.profile.Username == "" {
			a.returnTo = settings
			a.state = username
			a.nameInput = ""
			a.shareAfterName = true
			return
		}
		a.profile.Share = !a.profile.Share
	case 4:
		old := a.profile.Clone()
		a.profile.Metrics = !a.profile.Metrics
		if a.save == nil || !a.persist() {
			a.profile = old
			a.notice = "Usage sharing requires a saved profile."
			return
		}
		a.configureMetrics()
		return
	}
	a.persist()
	if a.setting == 3 && a.profile.Share {
		a.sync(a.entry().ID)
	}
}
func (a *App) launch(newGame func() engine.Game) {
	a.notice = ""
	a.game = newGame()
	if a.profile.Metrics && a.metrics != nil && profile.ValidGame(a.activity) {
		a.game = metrics.Track(a.game, a.activity, a.metrics)
	}
	a.active = engine.New(a.game)
	a.active.Resize(a.width, a.height)
}
func (a *App) Advance(dt time.Duration) {
	a.poll()
	if a.active != nil {
		a.active.Advance(dt)
		a.observeScore()
	}
}
