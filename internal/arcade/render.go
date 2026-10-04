package arcade

import (
	"fmt"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/ui"
	"strings"
)

func (a *App) Render(raw engine.Canvas) {
	c := ui.Canvas{Canvas: raw, Language: a.profile.Language}
	if a.active != nil {
		a.active.Render(c)
		if a.active.Ready() {
			w, h := c.Size()
			c.Text(0, h-1, strings.Repeat(" ", w), engine.Default)
			footer := activityFooter
			if a.activity == "diagnostic" {
				footer = " Q / Esc: back to menu   Ctrl+C: quit DevCade"
			}
			c.Text(0, h-1, footer, engine.Accent)
		}
		return
	}
	if !a.menuReady() {
		engine.RenderTooSmall(c, MenuWidth, MenuHeight, a.width, a.height)
		return
	}
	w, h := c.Size()
	c.Text(1, 0, "DEVCADE >_  terminal arcade", engine.Accent)
	c.Text(1, 1, "Quick games for the wait while builds, tests or AI agents run.", engine.Default)
	c.Text(0, 2, strings.Repeat("-", w), engine.Default)
	text := func(y int, s string, color engine.Color) { c.Text(2, y, ui.Clip(s, w-4), color) }
	option := func(y int, label string, selected bool) {
		marker, color := "   ", engine.Default
		if selected {
			marker, color = " > ", engine.Player
		}
		text(y, marker+ui.Translate(a.profile.Language, label), color)
	}
	switch a.state {
	case mainMenu:
		text(4, "GAMES", engine.Accent)
		optionY := max(10, 6+a.catalog.Len())
		creatorY := optionY + 1
		descriptionY := creatorY + 2
		toolsY := max(17, descriptionY+2)
		for i := range a.catalog.Len() {
			e := a.catalog.Entry(i)
			marker, color := "   ", engine.Default
			if i == a.selected {
				marker, color = " > ", engine.Player
			}
			text(5+i, fmt.Sprintf("%s%-12s %s", marker, e.Name, ui.Translate(a.profile.Language, e.Status())), color)
		}
		option(optionY, "Settings", a.selected == a.catalog.Len())
		option(creatorY, "Open creator profile", a.selected == a.catalog.Len()+1)
		if a.selected < a.catalog.Len() {
			text(descriptionY, ui.Translate(a.profile.Language, a.entry().Description), engine.Default)
		}
		text(toolsY, "TOOLS", engine.Accent)
		text(toolsY+1, "   D  Terminal diagnostic: moving @ to check input, timing and resize", engine.Default)
		text(20, "Built by cagridursun (Twitter: c__dursun)", engine.Accent)
		text(21, TwitterURL, engine.Default)
		c.Text(0, h-1, menuFooterPlay, engine.Accent)
	case gameMenu:
		text(4, a.entry().Name, engine.Accent)
		text(6, a.bestText(c), engine.Default)
		option(9, "New game", a.choice == 0)
		option(10, "Leaderboard", a.choice == 1)
		text(13, ui.Translate(a.profile.Language, a.entry().Description), engine.Default)
		c.Text(0, h-1, " Up/Down: select  Enter: confirm  Q / Esc: back", engine.Accent)
	case settings:
		text(4, "Settings", engine.Accent)
		lname := a.profile.Language
		for i, l := range ui.Languages {
			if l == a.profile.Language {
				lname = ui.LanguageNames[i]
			}
		}
		tname := a.profile.Theme
		for i, l := range ui.Themes {
			if l == a.profile.Theme {
				tname = ui.Translate(a.profile.Language, ui.ThemeNames[i])
			}
		}
		name := a.profile.Username
		if name == "" {
			name = ui.Translate(a.profile.Language, "Guest")
		}
		sharing := "Off"
		if a.profile.Share {
			sharing = "On"
		}
		usage := "Off"
		if a.profile.Metrics {
			usage = "On"
		}
		labels := []string{ui.Translate(a.profile.Language, "Language") + ": " + lname, ui.Translate(a.profile.Language, "Color palette") + ": " + tname, ui.Translate(a.profile.Language, "Username") + ": " + name, ui.Translate(a.profile.Language, "Global score sharing") + ": " + ui.Translate(a.profile.Language, sharing), ui.Translate(a.profile.Language, "Usage statistics") + ": " + ui.Translate(a.profile.Language, usage)}
		for i, s := range labels {
			option(6+i, s, i == a.setting)
		}
		for i, line := range ui.Wrap(ui.Translate(a.profile.Language, "Sharing publishes your username and best scores. Existing rows stay public."), w-4) {
			text(12+i, line, engine.Default)
		}
		for i, line := range ui.Wrap(ui.Translate(a.profile.Language, "Username changes create a new online identity; old records stay public."), w-4) {
			text(15+i, line, engine.Default)
		}
		for i, line := range ui.Wrap(ui.Translate(a.profile.Language, "Shares a random ID, game, score, duration, result, version and platform."), w-4) {
			text(17+i, line, engine.Default)
		}
		text(19, ui.Translate(a.profile.Language, a.networkNotice), engine.Warning)
		c.Text(0, h-1, " Up/Down: select  Left/Right/Enter: change  Q / Esc: back", engine.Accent)
	case username:
		text(4, "Choose your username", engine.Accent)
		text(6, "Use 3-20 lowercase letters, numbers or underscores.", engine.Default)
		text(8, "> "+a.nameInput+"_", engine.Player)
		text(11, "A username is not a verified GitHub or X account.", engine.Default)
		text(13, "Enable sharing in Settings to publish your name and personal bests.", engine.Default)
		footer := " Enter: save locally   Backspace: erase   Esc: play as guest"
		if a.profile.Username != "" {
			footer = " Enter: save   Backspace: erase   Esc: cancel"
		}
		if a.profile.Share || a.shareAfterName {
			text(15, "Sharing is on: your alias and saved bests will be published.", engine.Warning)
			footer = " Enter: save and share   Backspace: erase   Esc: cancel"
		}
		c.Text(0, h-1, footer, engine.Accent)
	case scores:
		text(3, a.entry().Name+" / "+ui.Translate(a.profile.Language, "Global personal bests"), engine.Accent)
		text(4, a.bestText(c), engine.Default)
		text(5, "Rank   Player                 Best", engine.Accent)
		b := a.boards[a.entry().ID]
		if len(b.Rows) == 0 && !a.busy && a.networkNotice == "" {
			text(7, "No scores yet.", engine.Default)
		}
		start := min(a.scroll, len(b.Rows))
		for i, r := range b.Rows[start:min(len(b.Rows), start+12)] {
			text(6+i, fmt.Sprintf("%4d   %-20s   %d", r.Rank, r.Username, r.Score), engine.Default)
		}
		if b.Own != nil {
			text(19, engine.Format(c, "You: #%d  %s  %d", b.Own.Rank, b.Own.Username, b.Own.Score), engine.Player)
		}
		text(20, "Community scores are client-reported.", engine.Default)
		text(21, ui.Translate(a.profile.Language, a.networkNotice), engine.Warning)
		c.Text(0, h-1, " Up/Down: scroll  R: refresh  Q / Esc: back", engine.Accent)
	}
	if a.notice != "" {
		text(22, ui.Translate(a.profile.Language, a.notice), engine.Warning)
	}
}
func (a *App) bestText(c engine.Canvas) string {
	if n, ok := a.profile.Best[a.entry().ID]; ok {
		return engine.Format(c, "Your best: %d", n)
	}
	return engine.Format(c, "Your best: -")
}
