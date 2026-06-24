package garmin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type consumer struct {
	key    string
	secret string
}

func (c consumer) authHeader(method, baseURL string, query url.Values, token, tokenSecret string) (string, error) {
	oauth := map[string]string{
		"oauth_consumer_key":     c.key,
		"oauth_nonce":            nonce(),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_version":          "1.0",
	}
	if token != "" {
		oauth["oauth_token"] = token
	}

	params := url.Values{}
	for k, v := range oauth {
		params.Set(k, v)
	}
	for k, vs := range query {
		for _, v := range vs {
			params.Add(k, v)
		}
	}
	base := signatureBaseString(method, baseURL, params)
	signingKey := pctEncode(c.secret) + "&" + pctEncode(tokenSecret)
	oauth["oauth_signature"] = hmacSHA1(base, signingKey)

	keys := make([]string, 0, len(oauth))
	for k := range oauth {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("OAuth ")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=\"%s\"", pctEncode(k), pctEncode(oauth[k]))
	}
	return b.String(), nil
}

func pctEncode(s string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if strings.IndexByte(unreserved, ch) >= 0 {
			b.WriteByte(ch)
		} else {
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

func nonce() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func signatureBaseString(method, baseURL string, params url.Values) string {
	var pairs []string
	for k, vs := range params {
		for _, v := range vs {
			pairs = append(pairs, pctEncode(k)+"="+pctEncode(v))
		}
	}
	sort.Strings(pairs)
	return strings.ToUpper(method) + "&" + pctEncode(baseURL) + "&" + pctEncode(strings.Join(pairs, "&"))
}

func hmacSHA1(message, key string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
