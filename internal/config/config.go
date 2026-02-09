package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// StringsConfig holds all the configurable strings for the BBS.
// This is the single source of truth.
type StringsConfig struct {
	ConnectionStr     string `json:"connectionStr"`
	LockedBaudStr     string `json:"lockedBaudStr"`
	ApplyAsNewStr     string `json:"applyAsNewStr"`
	GetNupStr         string `json:"getNupStr"`
	ChatRequestStr    string `json:"chatRequestStr"`
	LeaveFBStr        string `json:"leaveFBStr"`
	QuoteTitle        string `json:"quoteTitle"`
	QuoteMessageStr   string `json:"quoteMessageStr"`
	QuoteStartLine    string `json:"quoteStartLine"`
	QuoteEndLine      string `json:"quoteEndLine"`
	Erase5MsgsStr     string `json:"erase5MsgsStr"`
	ChangeBoardStr    string `json:"changeBoardStr"`
	NewscanBoardStr   string `json:"newscanBoardStr"`
	PostOnBoardStr    string `json:"postOnBoardStr"`
	MsgTitleStr       string `json:"msgTitleStr"`
	MsgToStr          string `json:"msgToStr"`
	UploadMsgStr      string `json:"uploadMsgStr"`
	MsgAnonStr        string `json:"msgAnonStr"`
	SlashStr          string `json:"slashStr"`
	NewScanningStr    string `json:"newScanningStr"`
	ChangeFileAreaStr string `json:"changeFileAreaStr"`
	LogOffStr         string `json:"logOffStr"`
	ChangeAutoMsgStr  string `json:"changeAutoMsgStr"`
	NewUserNameStr    string `json:"newUserNameStr"`
	CreateAPassword   string `json:"createAPassword"`
	PauseString       string `json:"pauseString"`
	WhatsYourAlias    string `json:"whatsYourAlias"`
	WhatsYourPw       string `json:"whatsYourPw"`
	SysopWorkingStr   string `json:"sysopWorkingStr"`
	SysopInDos        string `json:"sysopInDos"`
	SystemPasswordStr string `json:"systemPasswordStr"`
	DefPrompt         string `json:"defPrompt"`
	EnterChat         string `json:"enterChat"`
	ExitChat          string `json:"exitChat"`
	SysOpIsIn         string `json:"sysOpIsIn"`
	SysOpIsOut        string `json:"sysOpIsOut"`
	HeaderStr         string `json:"headerStr"`
	InfoformPrompt    string `json:"infoformPrompt"`
	NewInfoFormPrompt string `json:"newInfoFormPrompt"`
	UserNotFound      string `json:"userNotFound"`
	DesignNewPrompt   string `json:"designNewPrompt"`
	YourCurrentPrompt string `json:"yourCurrentPrompt"`
	WantHotKeys       string `json:"wantHotKeys"`
	WantRumors        string `json:"wantRumors"`
	YourUserNum       string `json:"yourUserNum"`
	WelcomeNewUser    string `json:"welcomeNewUser"`
	EnterNumberHeader string `json:"enterNumberHeader"`
	EnterNumber       string `json:"enterNumber"`
	EnterUserNote     string `json:"enterUserNote"`
	CurFileArea       string `json:"curFileArea"`
	EnterRealName     string `json:"enterRealName"`
	ReEnterPassword   string `json:"reEnterPassword"`
	QuoteTop          string `json:"quoteTop"`
	QuoteBottom       string `json:"quoteBottom"`
	AskOneLiner       string `json:"askOneLiner"`
	EnterOneLiner     string `json:"enterOneLiner"`
	NewScanDateStr    string `json:"newScanDateStr"`
	AddBatchPrompt    string `json:"addBatchPrompt"`
	ListUsers         string `json:"listUsers"`
	ViewArchivePrompt string `json:"viewArchivePrompt"`
	AreaMsgNewScan    string `json:"areaMsgNewScan"`
	GetInfoPrompt     string `json:"getInfoPrompt"`
	MsgNewScanPrompt  string `json:"msgNewScanPrompt"`
	TypeFilePrompt    string `json:"typeFilePrompt"`
	ConfPrompt        string `json:"confPrompt"`
	FileListPrompt    string `json:"fileListPrompt"`
	UploadFileStr     string `json:"uploadFileStr"`
	DownloadStr       string `json:"downloadStr"`
	ListRange         string `json:"listRange"`
	ContinueStr       string `json:"continueStr"`
	ViewWhichForm     string `json:"viewWhichForm"`
	CheckingPhoneNum  string `json:"checkingPhoneNum"`
	CheckingUserBase  string `json:"checkingUserBase"`
	NameAlreadyUsed   string `json:"nameAlreadyUsed"`
	InvalidUserName   string `json:"invalidUserName"`
	SysPwIs           string `json:"sysPwIs"`
	NotValidated      string `json:"notValidated"`
	HaveMail          string `json:"haveMail"`
	ReadMailNow       string `json:"readMailNow"`
	DeleteNotice      string `json:"deleteNotice"`
	HaveFeedback      string `json:"haveFeedback"`
	ReadFeedback      string `json:"readFeedback"`
	LoginNow          string `json:"loginNow"`
	NewUsersWaiting   string `json:"newUsersWaiting"`
	VoteOnNewUsers    string `json:"voteOnNewUsers"`
	WrongPassword     string `json:"wrongPassword"`

	// Added field for message menu prompt
	MessageMenuPrompt string `json:"messageMenuPrompt"`

	// Added from Page 5
	AddBBSName          string `json:"addBBSName"`
	AddBBSNumber        string `json:"addBBSNumber"`
	AddBBSBaud          string `json:"addBBSBaud"`
	AddBBSSoftware      string `json:"addBBSSoftware"`
	AddExtendedBBSDescr string `json:"addExtendedBBSDescr"`
	BBSEntryAdded       string `json:"bbsEntryAdded"`
	ViewNextDescrip     string `json:"viewNextDescrip"`
	JoinedMsgConf       string `json:"joinedMsgConf"`
	JoinedFileConf      string `json:"joinedFileConf"`
	WhosBeingVotedOn    string `json:"whosBeingVotedOn"`
	NumYesVotes         string `json:"numYesVotes"`
	NumNoVotes          string `json:"numNoVotes"`
	NUVCommentHeader    string `json:"nuvCommentHeader"`

	// Added from Page 6
	EnterNUVCommentPrompt string `json:"enterNUVCommentPrompt"`
	NUVVotePrompt         string `json:"nuvVotePrompt"`
	YesVoteCast           string `json:"yesVoteCast"`
	NoVoteCast            string `json:"noVoteCast"`
	NoNewUsersPending     string `json:"noNewUsersPending"`
	EnterRumorTitle       string `json:"enterRumorTitle"`
	AddRumorAnonymous     string `json:"addRumorAnonymous"`
	EnterRumorLevel       string `json:"enterRumorLevel"`
	EnterRumorPrompt      string `json:"enterRumorPrompt"`
	RumorAdded            string `json:"rumorAdded"`
	ListRumorsPrompt      string `json:"listRumorsPrompt"`
	SendMailToWho         string `json:"sendMailToWho"`
	CarbonCopyMail        string `json:"carbonCopyMail"`
	NotifyEMail           string `json:"notifyEMail"`
	EMailAnnouncement     string `json:"eMailAnnouncement"`
	SysOpNotHere          string `json:"sysOpNotHere"`
	ChatCostsHeader       string `json:"chatCostsHeader"`
	StillWantToTry        string `json:"stillWantToTry"`
	NotEnoughFPPoints     string `json:"notEnoughFPPoints"`
	ChatCallOff           string `json:"chatCallOff"`

	// Added from Page 7
	ChatCallOn          string `json:"chatCallOn"`
	FeedbackSent        string `json:"feedbackSent"`
	YouHaveReadMail     string `json:"youHaveReadMail"`
	DeleteMailNow       string `json:"deleteMailNow"`
	CurrentMailNone     string `json:"currentMailNone"`
	CurrentMailWaiting  string `json:"currentMailWaiting"`
	PickMailHeader      string `json:"pickMailHeader"`
	ListTitleType       string `json:"listTitleType"`
	NoMoreTitles        string `json:"noMoreTitles"`
	ListTitlesToYou     string `json:"listTitlesToYou"`
	SubDoesNotExist     string `json:"subDoesNotExist"`
	MsgNewScanAborted   string `json:"msgNewScanAborted"`
	MsgReadingPrompt    string `json:"msgReadingPrompt"`
	CurrentSubNewScan   string `json:"currentSubNewScan"`
	JumpToMessageNum    string `json:"jumpToMessageNum"`
	PostingQWKMsg       string `json:"postingQWKMsg"`
	TotalQWKAdded       string `json:"totalQWKAdded"`
	SendQWKPacketPrompt string `json:"sendQWKPacketPrompt"`

	// Added from Page 8
	ThreadWhichWay       string `json:"threadWhichWay"`
	AutoValidatingFile   string `json:"autoValidatingFile"`
	FileIsWorth          string `json:"fileIsWorth"`
	GrantingUserFP       string `json:"grantingUserFP"`
	FileIsOffline        string `json:"fileIsOffline"`
	CrashedFile          string `json:"crashedFile"`
	BadBaudRate          string `json:"badBaudRate"`
	UnvalidatedFile      string `json:"unvalidatedFile"`
	SpecialFile          string `json:"specialFile"`
	NoDownloadsHere      string `json:"noDownloadsHere"`
	PrivateFile          string `json:"privateFile"`
	FilePassword         string `json:"filePassword"`
	WrongFilePW          string `json:"wrongFilePW"`
	FileNewScanPrompt    string `json:"fileNewScanPrompt"`
	InvalidArea          string `json:"invalidArea"`
	UntaggingBatchFile   string `json:"untaggingBatchFile"`
	FileExtractionPrompt string `json:"fileExtractionPrompt"`

	// Added from Page 9
	BadUDRatio             string `json:"badUDRatio"`
	BadUDKRatio            string `json:"badUDKRatio"`
	ExceededDailyKBLimit   string `json:"exceededDailyKBLimit"`
	FilePointCommision     string `json:"filePointCommision"`
	SuccessfulDownload     string `json:"successfulDownload"`
	FileCrashSave          string `json:"fileCrashSave"`
	InvalidFilename        string `json:"invalidFilename"`
	AlreadyEnteredFilename string `json:"alreadyEnteredFilename"`
	FileAlreadyExists      string `json:"fileAlreadyExists"`
	EnterFileDescription   string `json:"enterFileDescription"`
	ExtendedUploadSetup    string `json:"extendedUploadSetup"`
	ReEnterFileDescrip     string `json:"reEnterFileDescrip"`
	NotifyIfDownloaded     string `json:"notifyIfDownloaded"`
	FiftyFilesMaximum      string `json:"fiftyFilesMaximum"`
	YouCantDownloadHere    string `json:"youCantDownloadHere"`
	FileAlreadyMarked      string `json:"fileAlreadyMarked"`
	NotEnoughFP            string `json:"notEnoughFP"`
	FileAreaPassword       string `json:"fileAreaPassword"`
	QuotePrefix            string `json:"QuotePrefix"`

	// Default Colors (|C1 - |C7 map to these)
	DefColor1 uint8 `json:"defColor1"`
	DefColor2 uint8 `json:"defColor2"`
	DefColor3 uint8 `json:"defColor3"`
	DefColor4 uint8 `json:"defColor4"`
	DefColor5 uint8 `json:"defColor5"`
	DefColor6 uint8 `json:"defColor6"`
	DefColor7 uint8 `json:"defColor7"`
}

// GlobalStrings holds the loaded configuration.
// var GlobalStrings StringsConfig // Removed duplicate

// LoadStrings loads the string configuration from a JSON file.
func LoadStrings(configPath string) (StringsConfig, error) { // Return the loaded config directly
	filePath := filepath.Join(configPath, "strings.json")
	log.Printf("INFO: Loading strings configuration from %s", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("ERROR: Failed to read strings file %s: %v", filePath, err)
		return StringsConfig{}, fmt.Errorf("failed to read strings file %s: %w", filePath, err)
	}

	var loadedConfig StringsConfig // Load into a local variable
	err = json.Unmarshal(data, &loadedConfig)
	if err != nil {
		log.Printf("ERROR: Failed to parse strings JSON from %s: %v", filePath, err)
		return StringsConfig{}, fmt.Errorf("failed to parse strings JSON from %s: %w", filePath, err)
	}

	log.Printf("INFO: Successfully loaded strings configuration.")
	return loadedConfig, nil // Return the loaded struct
}

// DoorConfig defines the configuration for a single external door program.
type DoorConfig struct {
	Name                string            `json:"name"`                            // Unique identifier used in DOOR:NAME commands
	Command             string            `json:"command"`                         // Path to the executable
	Args                []string          `json:"args"`                            // Command-line arguments (can include placeholders)
	WorkingDirectory    string            `json:"working_directory,omitempty"`     // Directory to run the command in (optional)
	DropfileType        string            `json:"dropfile_type,omitempty"`         // Type of dropfile ("DOOR.SYS", "CHAIN.TXT", "NONE") (optional, defaults to NONE)
	IOMode              string            `json:"io_mode,omitempty"`               // I/O handling ("STDIO", "FOSSIL" - future) (optional, defaults to STDIO)
	RequiresRawTerminal bool              `json:"requires_raw_terminal,omitempty"` // Whether the BBS should attempt to put the terminal in raw mode (optional, defaults to false)
	EnvironmentVars     map[string]string `json:"environment_variables,omitempty"` // Additional environment variables (optional)
}

// LoadDoors loads the door configuration from the specified file path.
func LoadDoors(filePath string) (map[string]DoorConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		// If the file doesn't exist, return an empty map and no error, as doors are optional.
		if os.IsNotExist(err) {
			return make(map[string]DoorConfig), nil
		}
		return nil, fmt.Errorf("failed to read doors file %s: %w", filePath, err)
	}

	var doors []DoorConfig
	err = json.Unmarshal(data, &doors)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal doors JSON from %s: %w", filePath, err)
	}

	doorMap := make(map[string]DoorConfig)
	for _, door := range doors {
		if _, exists := doorMap[door.Name]; exists {
			return nil, fmt.Errorf("duplicate door name found in %s: %s", filePath, door.Name)
		}
		doorMap[door.Name] = door
	}

	return doorMap, nil
}

// LoadOneLiners loads oneliner strings from a JSON file.
func LoadOneLiners(dataPath string) ([]string, error) { // Changed configPath to dataPath for clarity
	filePath := filepath.Join(dataPath, "oneliners.dat") // Assume loading .dat from data path
	log.Printf("INFO: Loading oneliners from %s", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("WARN: oneliners.dat not found at %s. No oneliners loaded.", filePath)
			return []string{}, nil // Return empty slice, not an error
		}
		log.Printf("ERROR: Failed to read oneliners file %s: %v", filePath, err)
		return nil, fmt.Errorf("failed to read oneliners file %s: %w", filePath, err)
	}

	// Assuming oneliners.dat is plain text, one per line
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	// Filter out potential empty lines
	var oneLiners []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			oneLiners = append(oneLiners, line)
		}
	}

	log.Printf("INFO: Successfully loaded %d oneliners.", len(oneLiners))
	return oneLiners, nil
}

// ThemeConfig holds theme-related settings, loaded per menu set.
// Colors are standard DOS color codes (0-255).
type ThemeConfig struct {
	YesNoHighlightColor int `json:"yesNoHighlightColor"`
	YesNoRegularColor   int `json:"yesNoRegularColor"`
	// Add other theme elements here as needed (e.g., default menu colors)
}

// LoadThemeConfig loads theme settings from theme.json within a specific menu set path.
func LoadThemeConfig(menuSetPath string) (ThemeConfig, error) {
	filePath := filepath.Join(menuSetPath, "theme.json")
	log.Printf("INFO: Loading theme configuration from %s", filePath)

	// Default theme settings
	defaultTheme := ThemeConfig{
		YesNoHighlightColor: 112, // White on Black (inverse)
		YesNoRegularColor:   15,  // Bright White on Black
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("WARN: theme.json not found at %s. Using default theme settings.", filePath)
			return defaultTheme, nil // Return defaults if file doesn't exist
		}
		log.Printf("ERROR: Failed to read theme file %s: %v", filePath, err)
		return defaultTheme, fmt.Errorf("failed to read theme file %s: %w", filePath, err)
	}

	var theme ThemeConfig
	// Initialize theme with defaults before unmarshalling
	theme = defaultTheme
	err = json.Unmarshal(data, &theme)
	if err != nil {
		log.Printf("ERROR: Failed to parse theme JSON from %s: %v. Using default theme settings.", filePath, err)
		return defaultTheme, fmt.Errorf("failed to parse theme JSON from %s: %w", filePath, err)
	}

	log.Printf("INFO: Successfully loaded theme configuration from %s", filePath)
	return theme, nil
}

// ServerConfig defines server-wide settings
type ServerConfig struct {
	BoardName        string `json:"boardName"`
	BoardPhoneNumber string `json:"boardPhoneNumber"`
	SysOpLevel       int    `json:"sysOpLevel"`
	CoSysLevel       int    `json:"coSysLevel"`
	LogonLevel       int    `json:"logonLevel"`
	SSHPort          int    `json:"sshPort"`
	SSHHost          string `json:"sshHost"`
	SSHEnabled       bool   `json:"sshEnabled"`
	TelnetPort       int    `json:"telnetPort"`
	TelnetHost       string `json:"telnetHost"`
	TelnetEnabled    bool   `json:"telnetEnabled"`
}

// LoadServerConfig loads the server configuration from config.json
func LoadServerConfig(configPath string) (ServerConfig, error) {
	filePath := filepath.Join(configPath, "config.json")
	log.Printf("INFO: Loading server configuration from %s", filePath)

	// Default config values
	defaultConfig := ServerConfig{
		BoardName:        "ViSiON/3 BBS",
		BoardPhoneNumber: "",
		SysOpLevel:       255,
		CoSysLevel:       250,
		LogonLevel:       100,
		SSHPort:          2222,
		SSHHost:          "0.0.0.0",
		SSHEnabled:       true,
		TelnetPort:       2323,
		TelnetHost:       "0.0.0.0",
		TelnetEnabled:    false,
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("WARN: config.json not found at %s. Using default settings.", filePath)
			return defaultConfig, nil
		}
		return defaultConfig, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	var config ServerConfig
	// Initialize with defaults before unmarshalling
	config = defaultConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("ERROR: Failed to parse config JSON from %s: %v. Using default settings.", filePath, err)
		return defaultConfig, fmt.Errorf("failed to parse config JSON from %s: %w", filePath, err)
	}

	log.Printf("INFO: Successfully loaded server configuration from %s", filePath)
	return config, nil
}

// Add other shared config structs here later if needed.
