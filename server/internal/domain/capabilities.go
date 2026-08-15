package domain

// Capabilities 能力预检结果。
type Capabilities struct {
	NASAvailable       bool     `json:"nas_available"`
	NASIndexComplete   bool     `json:"nas_index_complete"`
	PansearchAvailable bool     `json:"pansearch_available"`
	LoggedInDrivers    []string `json:"logged_in_drivers"`
	NASPhase           string   `json:"nas_phase"`
	NASProcessedFiles  int      `json:"nas_processed_files"`
	NASTotalFiles      int      `json:"nas_total_files"`
	ServerVersion      string   `json:"server_version"`
}
