package context

import "time"

const (
	DefaultMaxTokens      = 200
	DefaultN              = 1
	DefaultTemperature    = 0.8
	maxRetries            = 3
	cacheLifeTime         = 24 * time.Hour
	ConversationCacheSize = 2
	DefaultPromptFile     = "pkg/openaiclient/context/config/prompt.json"
	GenerateResponse      = "Generating AI response for question: '%s', asked by user: '%s'"
)

type OpenAiContextConfig struct {
	APIKey                string
	Workers               int
	CacheLifeTime         time.Duration
	ConversationCacheSize int
	DefaultMaxTokens      int
	DefaultN              int
	DefaultTemperature    float64
	MaxRetries            int
	DefaultPromptFile     string
}

type TeamAdvisorConfig struct {
	Welcome               []string `json:"welcome"`
	AppInterface          []string `json:"app_interface"`
	AppSettings           []string `json:"app_settings"`
	BanningPhase          []string `json:"banning_phase"`
	PickingPhase          []string `json:"picking_phase"`
	ExampleQuestion       []string `json:"example_question"`
	QuickSettingsFeatures []string `json:"quick_settings_features"`
	AppSettingsDetails    []string `json:"app_settings_details"`
	BanningPhaseDetails   []string `json:"banning_phase_details"`
	PickingPhaseDetails   []string `json:"picking_phase_details"`
	TimeoutHandling       []string `json:"timeout_handling"`
}

type TeamAdvisorData struct {
	TeamAdvisorConfig `json:"team-advisor"`
}

type NormalPromptConfig struct {
	Welcome []string `json:"welcome"`
}

type NormalPromptData struct {
	NormalPromptConfig `json:"normal"`
}
