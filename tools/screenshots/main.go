package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"naomi.run/web"
)

type page struct {
	name string
	comp templ.Component
}

const coachName = "Naomi"

var pages = []page{
	{"login", web.Login(coachName)},
	{"conversation", web.Conversation(coachName, sampleConversation, 2, false)},
	{"settings", web.Settings(sampleSettings)},
	{"proposals", web.Proposals(coachName, sampleProposals)},
}

var sampleProposals = []web.Proposal{
	{ID: "1", Weekday: "Tue", Date: "Jun 30", Workout: "6 km easy + 6×20s strides", Detail: "Light turnover after Sunday's long run.", DistanceKm: 6, Rationale: "Keeps legs sharp without adding load."},
	{ID: "2", Weekday: "Thu", Date: "Jul 2", Workout: "5 × 1 km @ 10k pace", Detail: "First sharpening session for your 10k.", DistanceKm: 8, Rationale: "Six weeks out — time to touch race pace."},
}

var sampleConversation = []web.Message{
	{Role: web.RoleAssistant, Content: "Morning! Your 8 km easy run synced overnight — nicely controlled, you held zone 2 the whole way. Garmin logged 7h 40m sleep and HRV back up to 68 ms, so recovery's looking solid. How did the legs feel?", Time: "7:02 AM"},
	{Role: web.RoleUser, Content: "Pretty good, a bit heavy at the start but loosened up after a couple of km.", Time: "7:14 AM"},
	{Role: web.RoleAssistant, Content: "That's normal the day after a long run. Here's where your load's sitting — right in the sweet spot, no need to back off.", Time: "7:15 AM", Stats: []web.Stat{
		{Label: "Weekly", Value: "42km", Hint: "+3 vs last"},
		{Label: "ACWR", Value: "1.1", Hint: "balanced"},
		{Label: "Sleep", Value: "7:40", Hint: "HRV 68"},
	}},
	{Role: web.RoleUser, Content: "Nice. I've got a 10k race in 6 weeks I want to target.", Time: "7:20 AM"},
	{Role: web.RoleAssistant, Content: "Noted — I've recorded that 10k goal. Six weeks is enough to sharpen. I've put tomorrow's session on your watch to get us started:", Time: "7:21 AM", Workout: &web.WorkoutCard{
		When:     "Tomorrow · Tue",
		Name:     "Easy + strides",
		Distance: "6 km",
		Detail:   "6 × 20s strides @ 5k effort, full recovery",
		Pushed:   true,
	}},
	{Role: web.RoleUser, Content: "Perfect, thanks Naomi.", Time: "7:22 AM", Seen: true},
}

var sampleSettings = web.SettingsData{
	DisplayName:     "Dylan",
	StravaConnected: true,
	GarminConnected: true,
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("screenshots: %v", err)
	}
}

func run() error {
	outDir := flag.String("out", "../docs/screenshots", "output directory for screenshots")
	browserDir := flag.String("browser-dir", "../.cache/rod", "directory for Chromium")
	width := flag.Int("width", 390, "viewport width")
	height := flag.Int("height", 844, "viewport height")
	scale := flag.Float64("scale", 2, "device scale factor")
	settle := flag.Duration("settle", 2500*time.Millisecond, "wait after load for runtime CSS to apply")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	comps := make(map[string]templ.Component, len(pages))
	for _, p := range pages {
		comps[p.name] = p.comp
	}

	assets := http.StripPrefix("/assets/", http.FileServer(http.FS(web.Assets())))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			assets.ServeHTTP(w, r)
			return
		}
		comp, ok := comps[strings.Trim(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if err := comp.Render(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	binPath := os.Getenv("CHROME_BIN")
	if binPath == "" {
		bin := launcher.NewBrowser()
		bin.RootDir = *browserDir
		p, err := bin.Get()
		if err != nil {
			return fmt.Errorf("provision browser: %w", err)
		}
		binPath = p
	}

	l := launcher.New().Bin(binPath).Headless(true)
	if os.Getenv("CI") != "" {
		l = l.Set("no-sandbox")
	}
	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}
	defer func() { _ = browser.Close() }()

	tab, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return err
	}
	if err := (proto.EmulationSetDeviceMetricsOverride{
		Width:             *width,
		Height:            *height,
		DeviceScaleFactor: *scale,
		Mobile:            false,
	}).Call(tab); err != nil {
		return err
	}

	for _, p := range pages {
		if err := tab.Navigate(srv.URL + "/" + p.name); err != nil {
			return err
		}
		if err := tab.WaitLoad(); err != nil {
			return err
		}
		if err := tab.WaitIdle(10 * time.Second); err != nil {
			return err
		}
		time.Sleep(*settle)

		img, err := tab.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
		if err != nil {
			return err
		}
		out := filepath.Join(*outDir, p.name+".png")
		if err := os.WriteFile(out, img, 0o644); err != nil {
			return err
		}
		log.Printf("wrote %s", out)
	}
	return nil
}
