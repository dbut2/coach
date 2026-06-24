package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"naomi.run/clients/garmin"
	"naomi.run/database"
)

var errNotOwner = errors.New("planned workout not owned by user")

type planStructure struct {
	Steps []garmin.Step `json:"steps"`
}

func (s *Service) pushWorkout(c *gin.Context) {
	user := currentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad workout id"})
		return
	}
	if err := s.pushPlannedWorkout(c.Request.Context(), user.ID, id); err != nil {
		if errors.Is(err, errNotOwner) || errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workout not found"})
			return
		}
		log.Printf("push workout %s: %v", id, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not push to watch"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pushed"})
}

func (s *Service) unpushWorkout(c *gin.Context) {
	user := currentUser(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad workout id"})
		return
	}
	if err := s.unpushPlannedWorkout(c.Request.Context(), user.ID, id); err != nil {
		if errors.Is(err, errNotOwner) || errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workout not found"})
			return
		}
		log.Printf("unpush workout %s: %v", id, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not remove from watch"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

func (s *Service) pushPlannedWorkout(ctx context.Context, userID, id uuid.UUID) error {
	pw, err := s.q.GetPlannedWorkout(ctx, id)
	if err != nil {
		return err
	}
	if pw.UserID != userID {
		return errNotOwner
	}
	if !pw.Structure.Valid || len(pw.Structure.RawMessage) == 0 {
		return fmt.Errorf("planned workout %s has no structure to push", id)
	}
	var ps planStructure
	if err := json.Unmarshal(pw.Structure.RawMessage, &ps); err != nil {
		return fmt.Errorf("planned workout %s: bad structure: %w", id, err)
	}
	name := strings.TrimSpace(pw.WorkoutType.String)
	if name == "" {
		name = "Workout"
	}
	w, err := garmin.BuildWorkout(name, ps.Steps)
	if err != nil {
		return err
	}

	bearer, _, err := s.garminToken(ctx, userID)
	if err != nil {
		return err
	}

	workoutID := pw.GarminWorkoutID.Int64
	if pw.GarminWorkoutID.Valid {
		if err := s.garmin.UpdateWorkout(ctx, bearer, workoutID, w); err != nil {
			log.Printf("plan->watch: update %d failed, recreating: %v", workoutID, err)
			workoutID = 0
		}
	}
	if workoutID == 0 {
		newID, err := s.garmin.CreateWorkout(ctx, bearer, w)
		if err != nil {
			return err
		}
		workoutID = newID
	}

	scheduleID := pw.GarminScheduleID.Int64
	if !pw.GarminScheduleID.Valid {
		sid, err := s.garmin.ScheduleWorkout(ctx, bearer, workoutID, pw.ScheduledDate.Format("2006-01-02"))
		if err != nil {
			return err
		}
		scheduleID = sid
	}

	return s.q.SetPlannedWorkoutGarmin(ctx, database.SetPlannedWorkoutGarminParams{
		ID:               id,
		GarminWorkoutID:  sql.NullInt64{Int64: workoutID, Valid: workoutID != 0},
		GarminScheduleID: sql.NullInt64{Int64: scheduleID, Valid: scheduleID != 0},
	})
}

func (s *Service) unpushPlannedWorkout(ctx context.Context, userID, id uuid.UUID) error {
	pw, err := s.q.GetPlannedWorkout(ctx, id)
	if err != nil {
		return err
	}
	if pw.UserID != userID {
		return errNotOwner
	}
	if !pw.GarminWorkoutID.Valid && !pw.GarminScheduleID.Valid {
		return nil
	}
	bearer, _, err := s.garminToken(ctx, userID)
	if err != nil {
		return err
	}
	if pw.GarminScheduleID.Valid {
		if err := s.garmin.UnscheduleWorkout(ctx, bearer, pw.GarminScheduleID.Int64); err != nil {
			log.Printf("plan->watch: unschedule %d: %v", pw.GarminScheduleID.Int64, err)
		}
	}
	if pw.GarminWorkoutID.Valid {
		if err := s.garmin.DeleteWorkout(ctx, bearer, pw.GarminWorkoutID.Int64); err != nil {
			log.Printf("plan->watch: delete %d: %v", pw.GarminWorkoutID.Int64, err)
		}
	}
	return s.q.SetPlannedWorkoutGarmin(ctx, database.SetPlannedWorkoutGarminParams{ID: id})
}
