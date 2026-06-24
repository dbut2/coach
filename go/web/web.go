package web

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
	Time    string
}

type SettingsData struct {
	DisplayName     string
	StravaConnected bool
	GarminConnected bool
	GarminState     string
}
