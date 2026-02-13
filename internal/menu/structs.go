package menu

// --- Record Structs (Modified for JSON Parsing) ---

// MenuRecord adapted for JSON parsing of .MNU files.
// Assumes JSON keys match the tags.
type MenuRecord struct {
	// Fields expected from JSON .MNU
	ClrScrBefore bool   `json:"CLR"`
	ClsScrBefore bool   `json:"CLS"`
	UsePrompt    bool   `json:"USEPROMPT"`
	Prompt1      string `json:"PROMPT1"`
	Prompt2      string `json:"PROMPT2"`
	Fallback     string `json:"FALLBACK"`
	ACS          string `json:"ACS"`
	Password     string `json:"PASS"`
}

// Getters for boolean fields (using the JSON bool types directly)
func (mr *MenuRecord) GetClrScrBefore() bool { return mr.ClrScrBefore || mr.ClsScrBefore }
func (mr *MenuRecord) GetUsePrompt() bool    { return mr.UsePrompt }
