package service

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"naomi.run/coach"
	"naomi.run/database"
	"naomi.run/web"
)

type proposedDay struct {
	Date        string  `json:"date"`
	Workout     string  `json:"workout"`
	Detail      string  `json:"detail"`
	DistanceKm  float64 `json:"distance_km"`
	DurationMin int     `json:"duration_min"`
}

func (s *Service) proposals(c *gin.Context) {
	user := currentUser(c)
	rows, err := s.q.ListProposalsByStatus(c.Request.Context(), database.ListProposalsByStatusParams{
		UserID: user.ID,
		Status: "proposed",
	})
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	items := make([]web.Proposal, 0, len(rows))
	for _, r := range rows {
		var d proposedDay
		_ = json.Unmarshal(r.ProposedDiff, &d)
		items = append(items, web.Proposal{
			ID:         r.ID.String(),
			Rationale:  r.Rationale.String,
			Date:       d.Date,
			Weekday:    weekday(d.Date),
			Workout:    d.Workout,
			Detail:     d.Detail,
			DistanceKm: d.DistanceKm,
		})
	}
	render(c, http.StatusOK, web.Proposals(s.cfg.CoachName, items))
}

func (s *Service) approveProposal(c *gin.Context) {
	user := currentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/plan/proposals")
		return
	}
	p, err := s.q.GetProposal(c.Request.Context(), id)
	if err != nil || p.UserID != user.ID || p.Status != "proposed" {
		c.Redirect(http.StatusSeeOther, "/plan/proposals")
		return
	}

	var d proposedDay
	if err := json.Unmarshal(p.ProposedDiff, &d); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	pd, err := d.toPlanDay()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if _, err := s.q.UpsertPlannedWorkout(c.Request.Context(), planDayParams(p.PlanID, user.ID, pd)); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := s.decideProposal(c, user.ID, id, "approved", p.ProposedDiff); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/plan/proposals")
}

func (s *Service) rejectProposal(c *gin.Context) {
	user := currentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/plan/proposals")
		return
	}
	if err := s.decideProposal(c, user.ID, id, "rejected", nil); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/plan/proposals")
}

func (s *Service) decideProposal(c *gin.Context, userID, id uuid.UUID, status string, applied json.RawMessage) error {
	var diff = database.DecideProposalParams{
		ID:        id,
		Status:    status,
		DecidedBy: sql.NullString{String: "runner", Valid: true},
		UserID:    userID,
	}
	if len(applied) > 0 {
		diff.AppliedDiff = pqtype.NullRawMessage{RawMessage: applied, Valid: true}
	}
	return s.q.DecideProposal(c.Request.Context(), diff)
}

func (s *Service) pendingProposals(c *gin.Context, userID uuid.UUID) int {
	rows, err := s.q.ListProposalsByStatus(c.Request.Context(), database.ListProposalsByStatusParams{
		UserID: userID,
		Status: "proposed",
	})
	if err != nil {
		return 0
	}
	return len(rows)
}

func (d proposedDay) toPlanDay() (coach.PlanDay, error) {
	t, err := time.Parse("2006-01-02", d.Date)
	if err != nil {
		return coach.PlanDay{}, err
	}
	return coach.PlanDay{
		Date:        t,
		WorkoutType: d.Workout,
		Description: d.Detail,
		DistanceM:   d.DistanceKm * 1000,
		DurationS:   d.DurationMin * 60,
	}, nil
}

func weekday(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return t.Format("Mon")
}
