package garmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	ssoBase   = "https://sso.garmin.com/sso"
	oauthBase = "https://connectapi.garmin.com/oauth-service/oauth"
	apiBase   = "https://connectapi.garmin.com"

	defaultConsumerKey    = "fc3e99d2-118c-44b8-8ae3-03370dde24c0"
	defaultConsumerSecret = "E08WAR897WEy2knn7aFBrvegVAf0AFdWBBF"

	ssoUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148"
	apiUserAgent = "com.garmin.android.apps.connectmobile"
)

var ErrMFARequired = errors.New("garmin: multi-factor code required")

var (
	csrfRe   = regexp.MustCompile(`name="_csrf"\s+value="([^"]+)"`)
	titleRe  = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	ticketRe = regexp.MustCompile(`embed\?ticket=([^"]+)"`)
)

type Client struct {
	cons consumer
	HTTP *http.Client
}

func New(consumerKey, consumerSecret string) *Client {
	if consumerKey == "" {
		consumerKey = defaultConsumerKey
	}
	if consumerSecret == "" {
		consumerSecret = defaultConsumerSecret
	}
	return &Client{
		cons: consumer{key: consumerKey, secret: consumerSecret},
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

type OAuth2Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"-"`
}

type Tokens struct {
	OAuth1Token  string
	OAuth1Secret string
	Access       OAuth2Token
	DisplayName  string
	FullName     string
}

type LoginFlow struct {
	c        *Client
	web      *http.Client
	email    string
	password string
}

func (c *Client) Login(email, password string) *LoginFlow {
	jar, _ := cookiejar.New(nil)
	return &LoginFlow{
		c:        c,
		web:      &http.Client{Timeout: 30 * time.Second, Jar: jar},
		email:    email,
		password: password,
	}
}

func signinParams() url.Values {
	return url.Values{
		"id":                              {"gauth-widget"},
		"embedWidget":                     {"true"},
		"gauthHost":                       {ssoBase + "/embed"},
		"service":                         {ssoBase + "/embed"},
		"source":                          {ssoBase + "/embed"},
		"redirectAfterAccountLoginUrl":    {ssoBase + "/embed"},
		"redirectAfterAccountCreationUrl": {ssoBase + "/embed"},
	}
}

func (f *LoginFlow) Start(ctx context.Context) (*Tokens, error) {
	embed := url.Values{"id": {"gauth-widget"}, "embedWidget": {"true"}, "gauthHost": {ssoBase}}
	if _, err := f.webGet(ctx, ssoBase+"/embed?"+embed.Encode(), ""); err != nil {
		return nil, fmt.Errorf("sso embed: %w", err)
	}

	signinURL := ssoBase + "/signin?" + signinParams().Encode()
	body, err := f.webGet(ctx, signinURL, ssoBase+"/embed?"+embed.Encode())
	if err != nil {
		return nil, fmt.Errorf("sso signin page: %w", err)
	}
	csrf, err := match(csrfRe, body, "csrf token")
	if err != nil {
		return nil, err
	}

	form := url.Values{
		"username": {f.email},
		"password": {f.password},
		"embed":    {"true"},
		"_csrf":    {csrf},
	}
	body, err = f.webPostForm(ctx, signinURL, signinURL, form)
	if err != nil {
		return nil, fmt.Errorf("sso signin: %w", err)
	}

	title := strings.TrimSpace(firstGroup(titleRe, body))
	if strings.Contains(strings.ToLower(title), "mfa") || strings.Contains(body, "verifyMFA") {
		return nil, ErrMFARequired
	}
	ticket, err := match(ticketRe, body, "service ticket")
	if err != nil {
		return nil, fmt.Errorf("login failed (check email/password): %w", err)
	}
	return f.finish(ctx, ticket)
}

func (f *LoginFlow) SubmitMFA(ctx context.Context, code string) (*Tokens, error) {
	signinURL := ssoBase + "/signin?" + signinParams().Encode()
	page, err := f.webGet(ctx, signinURL, signinURL)
	if err != nil {
		return nil, fmt.Errorf("mfa page: %w", err)
	}
	csrf, err := match(csrfRe, page, "mfa csrf token")
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"mfa-code": {strings.TrimSpace(code)},
		"embed":    {"true"},
		"fromPage": {"setupEnterMfaCode"},
		"_csrf":    {csrf},
	}
	mfaURL := ssoBase + "/verifyMFA/loginEnterMfaCode?" + signinParams().Encode()
	body, err := f.webPostForm(ctx, mfaURL, signinURL, form)
	if err != nil {
		return nil, fmt.Errorf("verify mfa: %w", err)
	}
	ticket, err := match(ticketRe, body, "service ticket")
	if err != nil {
		return nil, fmt.Errorf("mfa rejected (check the code): %w", err)
	}
	return f.finish(ctx, ticket)
}

func (f *LoginFlow) finish(ctx context.Context, ticket string) (*Tokens, error) {
	o1tok, o1sec, err := f.c.preauthorized(ctx, ticket)
	if err != nil {
		return nil, err
	}
	access, err := f.c.Exchange(ctx, o1tok, o1sec)
	if err != nil {
		return nil, err
	}
	prof, err := f.c.profile(ctx, access.AccessToken)
	if err != nil {
		return nil, err
	}
	return &Tokens{
		OAuth1Token:  o1tok,
		OAuth1Secret: o1sec,
		Access:       *access,
		DisplayName:  prof.DisplayName,
		FullName:     prof.FullName,
	}, nil
}

func (c *Client) preauthorized(ctx context.Context, ticket string) (string, string, error) {
	q := url.Values{
		"ticket":             {ticket},
		"login-url":          {ssoBase + "/embed"},
		"accepts-mfa-tokens": {"true"},
	}
	base := oauthBase + "/preauthorized"
	auth, err := c.cons.authHeader(http.MethodGet, base, q, "", "")
	if err != nil {
		return "", "", err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q.Encode(), nil)
	req.Header.Set("Authorization", auth)
	req.Header.Set("User-Agent", apiUserAgent)
	body, err := do(c.HTTP, req)
	if err != nil {
		return "", "", fmt.Errorf("preauthorized: %w", err)
	}
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return "", "", fmt.Errorf("preauthorized parse: %w", err)
	}
	tok, sec := vals.Get("oauth_token"), vals.Get("oauth_token_secret")
	if tok == "" || sec == "" {
		return "", "", fmt.Errorf("preauthorized: empty token in %q", string(body))
	}
	return tok, sec, nil
}

func (c *Client) Exchange(ctx context.Context, o1token, o1secret string) (*OAuth2Token, error) {
	base := oauthBase + "/exchange/user/2.0"
	auth, err := c.cons.authHeader(http.MethodPost, base, nil, o1token, o1secret)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base, strings.NewReader(""))
	req.Header.Set("Authorization", auth)
	req.Header.Set("User-Agent", apiUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := do(c.HTTP, req)
	if err != nil {
		return nil, fmt.Errorf("oauth2 exchange: %w", err)
	}
	var t OAuth2Token
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("oauth2 exchange parse: %w", err)
	}
	t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	return &t, nil
}

func (f *LoginFlow) webGet(ctx context.Context, u, referer string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	f.webHeaders(req, referer)
	body, err := do(f.web, req)
	return string(body), err
}

func (f *LoginFlow) webPostForm(ctx context.Context, u, referer string, form url.Values) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	f.webHeaders(req, referer)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := do(f.web, req)
	return string(body), err
}

func (f *LoginFlow) webHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", ssoUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func do(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return body, fmt.Errorf("status %d: %s", resp.StatusCode, snippet)
	}
	return body, nil
}

func match(re *regexp.Regexp, s, what string) (string, error) {
	if v := firstGroup(re, s); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("garmin: %s not found in response", what)
}

func firstGroup(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
