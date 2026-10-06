package arcade

import (
	"context"
	"errors"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/profile"
	"time"
)

func (a *App) openCreator() {
	if a.openURL == nil {
		a.notice = "Browser unavailable. Open https://x.com/c__dursun manually."
		return
	}
	if a.opening {
		return
	}
	a.opening = true
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
		defer cancel()
		err := a.openURL(ctx, TwitterURL)
		select {
		case a.social <- err:
		case <-a.ctx.Done():
		}
	}()
}
func (a *App) sync(game string) {
	if a.client == nil {
		a.networkNotice = "Global service is not configured. Your personal bests are saved locally."
		return
	}
	if a.profile.Share && a.save == nil {
		a.networkNotice = "Profile could not be saved. Changes apply only to this session."
		return
	}
	if a.profile.Share && !a.persist() {
		a.networkNotice = "Profile could not be saved. Changes apply only to this session."
		return
	}
	if !profile.ValidGame(game) {
		return
	}
	if a.busy {
		a.pending = true
		return
	}
	a.busy = true
	a.networkNotice = "Loading global scores..."
	p := a.profile.Clone()
	client := a.client
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
		defer cancel()
		out := result{game: game, name: p.Username}
		if p.Share && p.Username != "" {
			if !p.Identity.Valid(client.Endpoint) {
				registration, err := client.Register(ctx, p.Username)
				out.err = err
				if err == nil {
					out.registration = &registration
				}
				select {
				case a.results <- out:
				case <-a.ctx.Done():
				}
				return
			}
			for id, n := range p.Best {
				if err := client.Submit(ctx, p.Identity.Token, id, n); err != nil {
					out.err = err
					break
				}
			}
		}
		if out.err == nil {
			id := ""
			if p.Identity.Endpoint == client.Endpoint {
				id = p.Identity.ID
			}
			out.board, out.err = client.Fetch(ctx, game, id)
		}
		select {
		case a.results <- out:
		case <-a.ctx.Done():
		}
	}()
}
func (a *App) poll() {
	select {
	case err := <-a.social:
		a.opening = false
		if err != nil {
			a.notice = "Browser unavailable. Open https://x.com/c__dursun manually."
		}
	default:
	}
	select {
	case out := <-a.results:
		a.busy = false
		pending := a.pending
		a.pending = false
		if out.registration != nil {
			if a.profile.Username == out.name && a.profile.Share {
				a.profile.Identity = profile.Identity{ID: out.registration.ID, Token: out.registration.Token, Endpoint: a.client.Endpoint}
				if a.persist() {
					a.sync(out.game)
				}
			}
			return
		}
		switch {
		case errors.Is(out.err, leaderboard.ErrNameTaken):
			a.networkNotice = "Username already belongs to another online identity. Choose a different username; local bests are safe."
		case errors.Is(out.err, leaderboard.ErrIdentity):
			a.profile.Identity = profile.Identity{}
			a.persist()
			a.networkNotice = "Online identity expired. Choose a new username in Settings; local bests are safe and will sync after reconnecting."
		case out.err != nil:
			a.networkNotice = "Offline: global scores unavailable. Local bests are kept."
		default:
			if out.name != a.profile.Username {
				out.board.Own = nil
			}
			a.boards[out.game] = out.board
			a.scroll = min(a.scroll, max(0, len(out.board.Rows)-12))
			a.networkNotice = ""
		}
		if (pending && out.err == nil) || (a.state == scores && a.entry().ID != out.game) {
			a.sync(a.entry().ID)
		}
	default:
	}
}
