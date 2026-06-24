package garmin

import (
	"net/url"
	"strings"
	"testing"
)

func TestPctEncode(t *testing.T) {
	cases := map[string]string{
		" ":                  "%20",
		"=":                  "%3D",
		"azAZ09-._~":         "azAZ09-._~",
		"r b":                "r%20b",
		"=%3D":               "%3D%253D",
		"Smith & Jones":      "Smith%20%26%20Jones",
		"display@name.user1": "display%40name.user1",
	}
	for in, want := range cases {
		if got := pctEncode(in); got != want {
			t.Errorf("pctEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSignatureBaseString(t *testing.T) {
	params := url.Values{
		"b5":                     {"=%3D"},
		"a3":                     {"a", "2 q"},
		"c@":                     {""},
		"a2":                     {"r b"},
		"oauth_consumer_key":     {"9djdj82h48djs9d2"},
		"oauth_token":            {"kkk9d7dh3k39sjv7"},
		"oauth_signature_method": {"HMAC-SHA1"},
		"oauth_timestamp":        {"137131201"},
		"oauth_nonce":            {"7d8f3e4a"},
		"c2":                     {""},
	}
	want := "POST&http%3A%2F%2Fexample.com%2Frequest&" +
		"a2%3Dr%2520b%26a3%3D2%2520q%26a3%3Da%26b5%3D%253D%25253D%26c%2540%3D%26c2%3D" +
		"%26oauth_consumer_key%3D9djdj82h48djs9d2%26oauth_nonce%3D7d8f3e4a" +
		"%26oauth_signature_method%3DHMAC-SHA1%26oauth_timestamp%3D137131201" +
		"%26oauth_token%3Dkkk9d7dh3k39sjv7"
	if got := signatureBaseString("post", "http://example.com/request", params); got != want {
		t.Errorf("base string mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestAuthHeaderShape(t *testing.T) {
	c := consumer{key: "ckey", secret: "csecret"}
	h, err := c.authHeader("GET", oauthBase+"/preauthorized", url.Values{"ticket": {"ST-1"}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OAuth ", "oauth_consumer_key=\"ckey\"", "oauth_signature_method=\"HMAC-SHA1\"", "oauth_signature=", "oauth_nonce="} {
		if !strings.Contains(h, want) {
			t.Errorf("auth header %q missing %q", h, want)
		}
	}
	if strings.Contains(h, "oauth_token=") {
		t.Errorf("two-legged header should not carry oauth_token: %q", h)
	}
}

func TestWellnessHasData(t *testing.T) {
	if (Wellness{}).HasData() {
		t.Error("empty wellness should report no data")
	}
	rhr := 48
	w := Wellness{SleepSeconds: 27000, RestingHR: &rhr}
	if !w.HasData() {
		t.Error("wellness with sleep + RHR should report data")
	}
	if got := w.SleepMinutes(); got != 450 {
		t.Errorf("SleepMinutes = %d, want 450", got)
	}
}
