package strava

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

const (
	Source = "strava"
	Scope  = "read,activity:read_all"
)

func Config(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{Scope},
		Endpoint:     endpoints.Strava,
	}
}

type Athlete struct {
	ID        int64
	FirstName string
}

func AthleteFromToken(tok *oauth2.Token) Athlete {
	a, ok := tok.Extra("athlete").(map[string]any)
	if !ok {
		return Athlete{}
	}
	id, _ := a["id"].(float64)
	fn, _ := a["firstname"].(string)
	return Athlete{ID: int64(id), FirstName: fn}
}
