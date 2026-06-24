package garmin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) apiGet(ctx context.Context, bearer, path string, q url.Values, out any) (found bool, err error) {
	u := apiBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("User-Agent", apiUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return false, fmt.Errorf("garmin %s: status %d: %s", path, resp.StatusCode, snippet)
	}
	if out == nil || len(body) == 0 || string(body) == "null" {
		return false, nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, fmt.Errorf("garmin %s: decode: %w", path, err)
	}
	return true, nil
}

type profileResp struct {
	DisplayName string `json:"displayName"`
	FullName    string `json:"fullName"`
	UserName    string `json:"userName"`
}

func (c *Client) profile(ctx context.Context, bearer string) (*profileResp, error) {
	var p profileResp
	if _, err := c.apiGet(ctx, bearer, "/userprofile-service/socialProfile", nil, &p); err != nil {
		return nil, fmt.Errorf("social profile: %w", err)
	}
	if p.DisplayName == "" {
		return nil, fmt.Errorf("social profile: no display name")
	}
	return &p, nil
}

type Wellness struct {
	Date string

	SleepSeconds int
	DeepSeconds  int
	LightSeconds int
	RemSeconds   int
	AwakeSeconds int
	SleepScore   *int
	SleepQuality string

	RestingHR *int
	AvgStress *int
	MaxStress *int
	Steps     *int

	BodyBatteryHigh *int
	BodyBatteryLow  *int

	HRVLastNightAvg *int
	HRVWeeklyAvg    *int
	HRVStatus       string

	ReadinessScore    *int
	ReadinessLevel    string
	ReadinessFeedback string
	RecoveryHours     *int

	Raw json.RawMessage
}

func (w Wellness) HasData() bool {
	return w.SleepSeconds > 0 || w.RestingHR != nil || w.HRVLastNightAvg != nil ||
		w.ReadinessScore != nil || w.BodyBatteryHigh != nil || w.Steps != nil
}

func (w Wellness) SleepMinutes() int { return w.SleepSeconds / 60 }

func (c *Client) Wellness(ctx context.Context, bearer, displayName, date string) (*Wellness, error) {
	w := &Wellness{Date: date}
	raw := map[string]json.RawMessage{}

	var sleep struct {
		DTO struct {
			SleepTimeSeconds  int `json:"sleepTimeSeconds"`
			DeepSleepSeconds  int `json:"deepSleepSeconds"`
			LightSleepSeconds int `json:"lightSleepSeconds"`
			RemSleepSeconds   int `json:"remSleepSeconds"`
			AwakeSleepSeconds int `json:"awakeSleepSeconds"`
			SleepScores       struct {
				Overall struct {
					Value        *int   `json:"value"`
					QualifierKey string `json:"qualifierKey"`
				} `json:"overall"`
			} `json:"sleepScores"`
		} `json:"dailySleepDTO"`
	}
	q := url.Values{"date": {date}, "nonSleepBufferMinutes": {"60"}}
	if ok, err := c.apiGet(ctx, bearer, "/wellness-service/wellness/dailySleepData/"+url.PathEscape(displayName), q, &sleep); err != nil {
		return nil, err
	} else if ok {
		w.SleepSeconds = sleep.DTO.SleepTimeSeconds
		w.DeepSeconds = sleep.DTO.DeepSleepSeconds
		w.LightSeconds = sleep.DTO.LightSleepSeconds
		w.RemSeconds = sleep.DTO.RemSleepSeconds
		w.AwakeSeconds = sleep.DTO.AwakeSleepSeconds
		w.SleepScore = sleep.DTO.SleepScores.Overall.Value
		w.SleepQuality = sleep.DTO.SleepScores.Overall.QualifierKey
		if b, err := json.Marshal(sleep); err == nil {
			raw["sleep"] = b
		}
	}

	var daily struct {
		TotalSteps              *int `json:"totalSteps"`
		RestingHeartRate        *int `json:"restingHeartRate"`
		AverageStressLevel      *int `json:"averageStressLevel"`
		MaxStressLevel          *int `json:"maxStressLevel"`
		BodyBatteryHighestValue *int `json:"bodyBatteryHighestValue"`
		BodyBatteryLowestValue  *int `json:"bodyBatteryLowestValue"`
	}
	dq := url.Values{"calendarDate": {date}}
	if ok, err := c.apiGet(ctx, bearer, "/usersummary-service/usersummary/daily/"+url.PathEscape(displayName), dq, &daily); err != nil {
		return nil, err
	} else if ok {
		w.Steps = daily.TotalSteps
		w.RestingHR = daily.RestingHeartRate
		w.AvgStress = daily.AverageStressLevel
		w.MaxStress = daily.MaxStressLevel
		w.BodyBatteryHigh = daily.BodyBatteryHighestValue
		w.BodyBatteryLow = daily.BodyBatteryLowestValue
		if b, err := json.Marshal(daily); err == nil {
			raw["daily"] = b
		}
	}

	var hrv struct {
		Summary struct {
			LastNightAvg *int   `json:"lastNightAvg"`
			WeeklyAvg    *int   `json:"weeklyAvg"`
			Status       string `json:"status"`
		} `json:"hrvSummary"`
	}
	if ok, err := c.apiGet(ctx, bearer, "/hrv-service/hrv/"+date, nil, &hrv); err != nil {
		return nil, err
	} else if ok {
		w.HRVLastNightAvg = hrv.Summary.LastNightAvg
		w.HRVWeeklyAvg = hrv.Summary.WeeklyAvg
		w.HRVStatus = hrv.Summary.Status
		if b, err := json.Marshal(hrv); err == nil {
			raw["hrv"] = b
		}
	}

	var ready []struct {
		Score         *int   `json:"score"`
		Level         string `json:"level"`
		FeedbackShort string `json:"feedbackShort"`
		RecoveryTime  *int   `json:"recoveryTime"`
	}
	if ok, err := c.apiGet(ctx, bearer, "/metrics-service/metrics/trainingreadiness/"+date, nil, &ready); err != nil {
		return nil, err
	} else if ok && len(ready) > 0 {
		r := ready[0]
		w.ReadinessScore = r.Score
		w.ReadinessLevel = r.Level
		w.ReadinessFeedback = r.FeedbackShort
		w.RecoveryHours = r.RecoveryTime
		if b, err := json.Marshal(ready); err == nil {
			raw["readiness"] = b
		}
	}

	if b, err := json.Marshal(raw); err == nil {
		w.Raw = b
	}
	return w, nil
}
