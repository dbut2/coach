package service

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"naomi.run/database"
	"naomi.run/strava"
	"naomi.run/web"
)

const (
	sessionCookie = "coach_session"
	stateCookie   = "coach_oauth_state"
	sessionTTL    = 180 * 24 * time.Hour
	userKey       = "user"
)

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) index(c *gin.Context) {
	if _, ok := s.userFromSession(c); ok {
		c.Redirect(http.StatusFound, "/conversation")
		return
	}
	render(c, http.StatusOK, web.Login(s.cfg.CoachName))
}

func (s *Service) connectStrava(c *gin.Context) {
	state, err := randomToken()
	if err != nil {
		render(c, http.StatusInternalServerError, web.ErrorPage(s.cfg.CoachName, "We couldn't start sign-in. Please try again."))
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(stateCookie, state, int((10 * time.Minute).Seconds()), "/", "", true, true)
	c.Redirect(http.StatusFound, s.oauth.AuthCodeURL(state))
}

func (s *Service) stravaCallback(c *gin.Context) {
	if e := c.Query("error"); e != "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	state, err := c.Cookie(stateCookie)
	if err != nil || state == "" || state != c.Query("state") {
		render(c, http.StatusBadRequest, web.ErrorPage(s.cfg.CoachName, "Your sign-in link expired. Please try again."))
		return
	}
	c.SetCookie(stateCookie, "", -1, "/", "", true, true)

	code := c.Query("code")
	if code == "" {
		render(c, http.StatusBadRequest, web.ErrorPage(s.cfg.CoachName, "Sign-in didn't complete. Please try again."))
		return
	}

	tok, err := s.oauth.Exchange(c.Request.Context(), code)
	if err != nil {
		render(c, http.StatusBadGateway, web.ErrorPage(s.cfg.CoachName, "We couldn't reach Strava. Please try again."))
		return
	}

	athlete := strava.AthleteFromToken(tok)
	if !s.cfg.athleteAllowed(athlete.ID) {
		render(c, http.StatusForbidden, web.TurnedAway(s.cfg.CoachName))
		return
	}

	userID, err := s.upsertAthlete(c, tok, athlete)
	if err != nil {
		render(c, http.StatusInternalServerError, web.ErrorPage(s.cfg.CoachName, "We couldn't save your connection. Please try again."))
		return
	}

	if err := s.startSession(c, userID); err != nil {
		render(c, http.StatusInternalServerError, web.ErrorPage(s.cfg.CoachName, "We couldn't start your session. Please try again."))
		return
	}

	c.Redirect(http.StatusFound, "/conversation")
}

func (s *Service) upsertAthlete(c *gin.Context, tok *oauth2.Token, athlete strava.Athlete) (uuid.UUID, error) {
	ctx := c.Request.Context()

	conn, err := s.q.GetStravaConnectionByAthleteID(ctx, athlete.ID)
	switch {
	case err == nil:
		return conn.UserID, s.q.UpdateStravaTokens(ctx, database.UpdateStravaTokensParams{
			UserID:       conn.UserID,
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			ExpiresAt:    tok.Expiry,
			Scope:        strava.Scope,
		})
	case errors.Is(err, sql.ErrNoRows):
		user, err := s.q.CreateUser(ctx, sql.NullString{String: athlete.FirstName, Valid: athlete.FirstName != ""})
		if err != nil {
			return uuid.Nil, err
		}
		_, err = s.q.CreateStravaConnection(ctx, database.CreateStravaConnectionParams{
			UserID:       user.ID,
			AthleteID:    athlete.ID,
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			ExpiresAt:    tok.Expiry,
			Scope:        strava.Scope,
		})
		return user.ID, err
	default:
		return uuid.Nil, err
	}
}

func (s *Service) startSession(c *gin.Context, userID uuid.UUID) error {
	id, err := randomToken()
	if err != nil {
		return err
	}
	_, err = s.q.CreateSession(c.Request.Context(), database.CreateSessionParams{
		ID:        id,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionTTL),
	})
	if err != nil {
		return err
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, id, int(sessionTTL.Seconds()), "/", "", true, true)
	return nil
}

func (s *Service) logout(c *gin.Context) {
	if id, err := c.Cookie(sessionCookie); err == nil && id != "" {
		_ = s.q.DeleteSession(c.Request.Context(), id)
	}
	c.SetCookie(sessionCookie, "", -1, "/", "", true, true)
	c.Redirect(http.StatusFound, "/")
}

func (s *Service) userFromSession(c *gin.Context) (database.User, bool) {
	id, err := c.Cookie(sessionCookie)
	if err != nil || id == "" {
		return database.User{}, false
	}
	row, err := s.q.GetSessionUser(c.Request.Context(), id)
	if err != nil {
		return database.User{}, false
	}
	return row.User, true
}

func (s *Service) requireAuth(c *gin.Context) {
	user, ok := s.userFromSession(c)
	if !ok {
		c.Redirect(http.StatusFound, "/")
		c.Abort()
		return
	}
	c.Set(userKey, user)
	c.Next()
}

func currentUser(c *gin.Context) database.User {
	return c.MustGet(userKey).(database.User)
}
