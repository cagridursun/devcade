package arcade

import "github.com/cagridursun/devcade/internal/metrics"

func (a *App) configureMetrics() {
	if a.metrics == nil {
		return
	}
	a.metrics.Disable()
	if !a.profile.Metrics || a.save == nil {
		return
	}
	if len(a.profile.MetricsID) != 32 {
		id, err := metrics.NewID()
		if err != nil {
			return
		}
		a.profile.MetricsID = id
	}
	if !a.persist() {
		return
	}
	a.metrics.Enable(a.profile.MetricsID)
	a.metrics.Emit(metrics.Event{Kind: "app_open"})
}
func (a *App) endMetrics(outcome string) {
	if game, ok := a.game.(*metrics.TrackedGame); ok {
		game.End(outcome)
	}
}
