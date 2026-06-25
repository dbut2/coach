package web

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role
	Content string
	Time    string
	Tool    string
	Seen    bool // for the runner's own last message: coach has read it

	// Optional structured attachments rendered as cards beneath the text.
	// Both are nil/empty for plain messages, so existing callers are unaffected.
	Workout *WorkoutCard
	Stats   []Stat
}

// WorkoutCard is a session the coach surfaces inline — text stays primary,
// the card just makes the structured shape glanceable.
type WorkoutCard struct {
	When     string // "Tomorrow · Tue"
	Name     string // "Easy + strides"
	Distance string // "6 km"
	Detail   string // "6 × 20s strides @ 5k effort"
	Pushed   bool   // synced to the watch
}

// Stat is a single measured value shown on request (pace, load, zones…).
type Stat struct {
	Label string
	Value string
	Hint  string
}

type SettingsData struct {
	DisplayName     string
	StravaConnected bool
	GarminConnected bool
	GarminState     string
}

type Proposal struct {
	ID         string
	Rationale  string
	Date       string
	Weekday    string
	Workout    string
	Detail     string
	DistanceKm float64
}
