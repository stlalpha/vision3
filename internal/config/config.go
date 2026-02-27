package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StringsConfig holds all the configurable strings for the BBS.
// This is the single source of truth.
type StringsConfig struct {
	ConnectionStr           string `json:"connectionStr"`
	LockedBaudStr           string `json:"lockedBaudStr"`
	ApplyAsNewStr           string `json:"applyAsNewStr"`
	GetNupStr               string `json:"getNupStr"`
	ChatRequestStr          string `json:"chatRequestStr"`
	LeaveFBStr              string `json:"leaveFBStr"`
	QuoteTitle              string `json:"quoteTitle"`
	QuoteMessageStr         string `json:"quoteMessageStr"`
	QuoteStartLine          string `json:"quoteStartLine"`
	QuoteEndLine            string `json:"quoteEndLine"`
	Erase5MsgsStr           string `json:"erase5MsgsStr"`
	ChangeBoardStr          string `json:"changeBoardStr"`
	NewscanBoardStr         string `json:"newscanBoardStr"`
	PostOnBoardStr          string `json:"postOnBoardStr"`
	MsgTitleStr             string `json:"msgTitleStr"`
	MsgToStr                string `json:"msgToStr"`
	MsgAnonStr              string `json:"msgAnonStr"`
	AnonymousName           string `json:"anonymousName"`
	SlashStr                string `json:"slashStr"`
	NewScanningStr          string `json:"newScanningStr"`
	ChangeFileAreaStr       string `json:"changeFileAreaStr"`
	LogOffStr               string `json:"logOffStr"`
	ChangeAutoMsgStr        string `json:"changeAutoMsgStr"`
	NewUserNameStr          string `json:"newUserNameStr"`
	CreateAPassword         string `json:"createAPassword"`
	PauseString             string `json:"pauseString"`
	WhatsYourAlias          string `json:"whatsYourAlias"`
	WhatsYourPw             string `json:"whatsYourPw"`
	SysopWorkingStr         string `json:"sysopWorkingStr"`
	SysopInDos              string `json:"sysopInDos"`
	SystemPasswordStr       string `json:"systemPasswordStr"`
	DefPrompt               string `json:"defPrompt"`
	EnterChat               string `json:"enterChat"`
	ExitChat                string `json:"exitChat"`
	SysOpIsIn               string `json:"sysOpIsIn"`
	SysOpIsOut              string `json:"sysOpIsOut"`
	HeaderStr               string `json:"headerStr"`
	InfoformPrompt          string `json:"infoformPrompt"`
	NewInfoFormPrompt       string `json:"newInfoFormPrompt"`
	UserNotFound            string `json:"userNotFound"`
	DesignNewPrompt         string `json:"designNewPrompt"`
	YourCurrentPrompt       string `json:"yourCurrentPrompt"`
	WantHotKeys             string `json:"wantHotKeys"`
	WantRumors              string `json:"wantRumors"`
	YourUserNum             string `json:"yourUserNum"`
	WelcomeNewUser          string `json:"welcomeNewUser"`
	EnterNumberHeader       string `json:"enterNumberHeader"`
	EnterNumber             string `json:"enterNumber"`
	EnterUserNote           string `json:"enterUserNote"`
	CurFileArea             string `json:"curFileArea"`
	EnterRealName           string `json:"enterRealName"`
	ReEnterPassword         string `json:"reEnterPassword"`
	QuoteTop                string `json:"quoteTop"`
	QuoteBottom             string `json:"quoteBottom"`
	AskOneLiner             string `json:"askOneLiner"`
	OneLinerAnonymousPrompt string `json:"oneLinerAnonymousPrompt"`
	EnterOneLiner           string `json:"enterOneLiner"`
	OneLinerLegend          string `json:"One_Liner_Legend"`
	NewScanDateStr          string `json:"newScanDateStr"`
	AddBatchPrompt          string `json:"addBatchPrompt"`
	ListUsers               string `json:"listUsers"`
	ViewArchivePrompt       string `json:"viewArchivePrompt"`
	AreaMsgNewScan          string `json:"areaMsgNewScan"`
	GetInfoPrompt           string `json:"getInfoPrompt"`
	MsgNewScanPrompt        string `json:"msgNewScanPrompt"`
	TypeFilePrompt          string `json:"typeFilePrompt"`
	ConfPrompt              string `json:"confPrompt"`
	FileListPrompt          string `json:"fileListPrompt"`
	UploadFileStr           string `json:"uploadFileStr"`
	DownloadStr             string `json:"downloadStr"`
	ListRange               string `json:"listRange"`
	ContinueStr             string `json:"continueStr"`
	ViewWhichForm           string `json:"viewWhichForm"`
	CheckingPhoneNum        string `json:"checkingPhoneNum"`
	CheckingUserBase        string `json:"checkingUserBase"`
	NameAlreadyUsed         string `json:"nameAlreadyUsed"`
	InvalidUserName         string `json:"invalidUserName"`
	SysPwIs                 string `json:"sysPwIs"`
	NotValidated            string `json:"notValidated"`
	HaveMail                string `json:"haveMail"`
	ReadMailNow             string `json:"readMailNow"`
	DeleteNotice            string `json:"deleteNotice"`
	HaveFeedback            string `json:"haveFeedback"`
	ReadFeedback            string `json:"readFeedback"`
	LoginNow                string `json:"loginNow"`
	NewUsersWaiting         string `json:"newUsersWaiting"`
	VoteOnNewUsers          string `json:"voteOnNewUsers"`
	WrongPassword           string `json:"wrongPassword"`
	YesPromptText           string `json:"yesPromptText"`
	NoPromptText            string `json:"noPromptText"`
	AbortMessagePrompt      string `json:"abortMessagePrompt"`

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

	// Chat strings (V3-specific)
	ChatHeader        string `json:"chatHeader"`
	ChatSeparator     string `json:"chatSeparator"`
	ChatUserEntered   string `json:"chatUserEntered"`
	ChatUserLeft      string `json:"chatUserLeft"`
	ChatSystemPrefix  string `json:"chatSystemPrefix"`
	ChatMessageFormat string `json:"chatMessageFormat"`

	// Page strings (V3-specific)
	PageOnlineNodesHeader string `json:"pageOnlineNodesHeader"`
	PageNodeListEntry     string `json:"pageNodeListEntry"`
	PageWhichNodePrompt   string `json:"pageWhichNodePrompt"`
	PageMessagePrompt     string `json:"pageMessagePrompt"`
	PageMessageFormat     string `json:"pageMessageFormat"`
	PageSent              string `json:"pageSent"`
	PageCancelled         string `json:"pageCancelled"`
	PageInvalidNode       string `json:"pageInvalidNode"`
	PageSelfError         string `json:"pageSelfError"`
	PageNodeOffline       string `json:"pageNodeOffline"`

	// Newuser strings (V3-specific)
	NewUsersClosedStr       string `json:"newUsersClosedStr"`
	NewUserLocationPrompt   string `json:"newUserLocationPrompt"`
	NewUserPasswordTooShort string `json:"newUserPasswordTooShort"`
	NewUserPasswordMismatch string `json:"newUserPasswordMismatch"`
	NewUserInvalidRealName  string `json:"newUserInvalidRealName"`
	NewUserTooManyAttempts  string `json:"newUserTooManyAttempts"`
	NewUserAccountCreated   string `json:"newUserAccountCreated"`
	NewUserCreationError    string `json:"newUserCreationError"`
	NewUserMaybeAnotherTime string `json:"newUserMaybeAnotherTime"`

	// System stats strings (V3-specific)
	StatsBBSName     string `json:"statsBBSName"`
	StatsSysOp       string `json:"statsSysOp"`
	StatsVersion     string `json:"statsVersion"`
	StatsTotalUsers  string `json:"statsTotalUsers"`
	StatsTotalCalls  string `json:"statsTotalCalls"`
	StatsTotalMsgs   string `json:"statsTotalMessages"`
	StatsTotalFiles  string `json:"statsTotalFiles"`
	StatsActiveNodes string `json:"statsActiveNodes"`
	StatsDate        string `json:"statsDate"`
	StatsTime        string `json:"statsTime"`

	// User config strings (V3-specific)
	CfgToggleOn            string `json:"cfgToggleOn"`
	CfgToggleOff           string `json:"cfgToggleOff"`
	CfgToggleFormat        string `json:"cfgToggleFormat"`
	CfgSaveError           string `json:"cfgSaveError"`
	CfgScreenWidthPrompt   string `json:"cfgScreenWidthPrompt"`
	CfgScreenWidthInvalid  string `json:"cfgScreenWidthInvalid"`
	CfgScreenWidthSet      string `json:"cfgScreenWidthSet"`
	CfgScreenHeightPrompt  string `json:"cfgScreenHeightPrompt"`
	CfgScreenHeightInvalid string `json:"cfgScreenHeightInvalid"`
	CfgScreenHeightSet     string `json:"cfgScreenHeightSet"`
	CfgTermTypeSet         string `json:"cfgTermTypeSet"`
	CfgStringPrompt        string `json:"cfgStringPrompt"`
	CfgStringPromptCurrent string `json:"cfgStringPromptCurrent"`
	CfgStringUpdated       string `json:"cfgStringUpdated"`
	CfgCurrentPwPrompt     string `json:"cfgCurrentPwPrompt"`
	CfgIncorrectPw         string `json:"cfgIncorrectPw"`
	CfgPasswordChanged     string `json:"cfgPasswordChanged"`
	CfgColorSelectPrompt   string `json:"cfgColorSelectPrompt"`
	CfgColorInputPrompt    string `json:"cfgColorInputPrompt"`
	CfgColorInvalid        string `json:"cfgColorInvalid"`
	CfgColorSet            string `json:"cfgColorSet"`
	CfgCustomPromptHelp    string `json:"cfgCustomPromptHelp"`
	CfgViewScreenWidth     string `json:"cfgViewScreenWidth"`
	CfgViewScreenHeight    string `json:"cfgViewScreenHeight"`
	CfgViewTermType        string `json:"cfgViewTermType"`
CfgViewHotKeys         string `json:"cfgViewHotKeys"`
	CfgViewMorePrompts     string `json:"cfgViewMorePrompts"`
	CfgViewMsgHeader       string `json:"cfgViewMsgHeader"`
	CfgViewCustomPrompt    string `json:"cfgViewCustomPrompt"`
	CfgViewPromptColor     string `json:"cfgViewPromptColor"`
	CfgViewTextColor       string `json:"cfgViewTextColor"`
	CfgViewText2Color      string `json:"cfgViewText2Color"`
	CfgViewBarColor        string `json:"cfgViewBarColor"`
	CfgViewRealName        string `json:"cfgViewRealName"`
	CfgViewPhone           string `json:"cfgViewPhone"`
	CfgViewNote            string `json:"cfgViewNote"`
	CfgViewFileListMode    string `json:"cfgViewFileListMode"`
	CfgFileListModeSet     string `json:"cfgFileListModeSet"`

	// Message reader strings (V3-specific)
	MsgEndOfMessages     string `json:"msgEndOfMessages"`
	MsgFirstMessage      string `json:"msgFirstMessage"`
	MsgMailReplyDeferred string `json:"msgMailReplyDeferred"`
	MsgListDeferred      string `json:"msgListDeferred"`
	MsgHdrLoadError      string `json:"msgHdrLoadError"`
	MsgBoardInfoFormat   string `json:"msgBoardInfoFormat"`
	MsgScrollPercent     string `json:"msgScrollPercent"`
	MsgNewScanSuffix     string `json:"msgNewScanSuffix"`
	MsgReadingSuffix     string `json:"msgReadingSuffix"`
	MsgThreadPrompt      string `json:"msgThreadPrompt"`
	MsgNoThreadFound     string `json:"msgNoThreadFound"`
	MsgJumpPrompt        string `json:"msgJumpPrompt"`
	MsgInvalidMsgNum     string `json:"msgInvalidMsgNum"`
	MsgReplySubjectEmpty string `json:"msgReplySubjectEmpty"`
	MsgLaunchingEditor   string `json:"msgLaunchingEditor"`
	MsgReplyCancelled    string `json:"msgReplyCancelled"`
	MsgReplyError        string `json:"msgReplyError"`
	MsgReplySuccess      string `json:"msgReplySuccess"`
	MsgEditorError       string `json:"msgEditorError"`

	// Message scan strings (V3-specific)
	ScanDateLine            string `json:"scanDateLine"`
	ScanToLine              string `json:"scanToLine"`
	ScanFromLine            string `json:"scanFromLine"`
	ScanRangeLine           string `json:"scanRangeLine"`
	ScanUpdateLine          string `json:"scanUpdateLine"`
	ScanWhichLine           string `json:"scanWhichLine"`
	ScanAbortLine           string `json:"scanAbortLine"`
	ScanSelectionPrompt     string `json:"scanSelectionPrompt"`
	ScanDatePrompt          string `json:"scanDatePrompt"`
	ScanToPrompt            string `json:"scanToPrompt"`
	ScanFromPrompt          string `json:"scanFromPrompt"`
	ScanRangeStartPrompt    string `json:"scanRangeStartPrompt"`
	ScanRangeEndPrompt      string `json:"scanRangeEndPrompt"`
	ScanWhichPrompt         string `json:"scanWhichPrompt"`
	ScanHeader              string `json:"scanHeader"`
	ScanNoAreaSelected      string `json:"scanNoAreaSelected"`
	ScanNoMessages          string `json:"scanNoMessages"`
	ScanNoTaggedAreas       string `json:"scanNoTaggedAreas"`
	ScanAreaProgress        string `json:"scanAreaProgress"`
	ScanComplete            string `json:"scanComplete"`
	ScanLoginRequired       string `json:"scanLoginRequired"`
	ScanNoAreasAvailable    string `json:"scanNoAreasAvailable"`
	ScanNoAccessibleAreas   string `json:"scanNoAccessibleAreas"`
	ScanConfigSaved         string `json:"scanConfigSaved"`
	ScanConfigError         string `json:"scanConfigError"`
	ScanConfigLoginRequired string `json:"scanConfigLoginRequired"`

	// Message list strings (V3-specific)
	MsgListLoginRequired  string `json:"msgListLoginRequired"`
	MsgListNoAreaSelected string `json:"msgListNoAreaSelected"`
	MsgListAreaNotFound   string `json:"msgListAreaNotFound"`
	MsgListLoadError      string `json:"msgListLoadError"`
	MsgListNoMessages     string `json:"msgListNoMessages"`

	// File viewer strings (V3-specific)
	FileNoAreaSelected string `json:"fileNoAreaSelected"`
	FilePromptFormat   string `json:"filePromptFormat"`
	FileNotFoundFormat string `json:"fileNotFoundFormat"`
	FileLocateError    string `json:"fileLocateError"`
	FileViewingHeader  string `json:"fileViewingHeader"`
	FileEndOfFile      string `json:"fileEndOfFile"`
	FileMorePrompt     string `json:"fileMorePrompt"`
	FilePausePrompt    string `json:"filePausePrompt"`
	FileOpenError      string `json:"fileOpenError"`

	// Door handler strings (V3-specific)
	DoorDropfileError     string `json:"doorDropfileError"`
	DoorErrorFormat       string `json:"doorErrorFormat"`
	DoorLoginRequired     string `json:"doorLoginRequired"`
	DoorPrompt            string `json:"doorPrompt"`
	DoorNotFoundFormat    string `json:"doorNotFoundFormat"`
	DoorNoneConfigured    string `json:"doorNoneConfigured"`
	DoorTemplateError     string `json:"doorTemplateError"`
	DoorInfoLoginRequired string `json:"doorInfoLoginRequired"`

	// Matrix strings (V3-specific)
	MatrixDisconnecting       string `json:"matrixDisconnecting"`
	MatrixCheckAccessPrompt   string `json:"matrixCheckAccessPrompt"`
	MatrixUserNotFound        string `json:"matrixUserNotFound"`
	MatrixAccountValidated    string `json:"matrixAccountValidated"`
	MatrixAccountNotValidated string `json:"matrixAccountNotValidated"`
	IdleTimeout               string `json:"idleTimeout"`

	// Conference menu strings (V3-specific)
	ConfLoginRequired           string `json:"confLoginRequired"`
	ConfNoConferences           string `json:"confNoConferences"`
	ConfNotFound                string `json:"confNotFound"`
	ConfNoAccessibleAreas       string `json:"confNoAccessibleAreas"`
	ConfCurrentAreaFormat       string `json:"confCurrentAreaFormat"`
	ConfTemplateError           string `json:"confTemplateError"`
	ConfNavLoginRequired        string `json:"confNavLoginRequired"`
	ConfNoAccessibleConferences string `json:"confNoAccessibleConferences"`
	ConfNoAccessibleMsgAreas    string `json:"confNoAccessibleMsgAreas"`
	ConfAreaTemplateError       string `json:"confAreaTemplateError"`

	// Executor strings (V3-specific)
	ExecUnknownCommand      string `json:"execUnknownCommand"`
	ExecReadmailLogin       string `json:"execReadmailLogin"`
	ExecReadmailPlaceholder string `json:"execReadmailPlaceholder"`
	ExecDoorLogin           string `json:"execDoorLogin"`
	ExecDoorNotConfigured   string `json:"execDoorNotConfigured"`
	ExecGoodbye             string `json:"execGoodbye"`
	ExecStatsLogin          string `json:"execStatsLogin"`
	ExecStatsError          string `json:"execStatsError"`
	ExecOnelinerTemplateErr string `json:"execOnelinerTemplateErr"`
	ExecOnelinerColorError  string `json:"execOnelinerColorError"`
	ExecOnelinerWriteError  string `json:"execOnelinerWriteError"`
	ExecOnelinerAdded       string `json:"execOnelinerAdded"`
	ExecOnelinerEmpty       string `json:"execOnelinerEmpty"`
	ExecAlreadyLoggedIn     string `json:"execAlreadyLoggedIn"`
	ExecUsernamePrompt      string `json:"execUsernamePrompt"`
	ExecPasswordPrompt      string `json:"execPasswordPrompt"`
	ExecAbortLoginPrompt    string `json:"execAbortLoginPrompt"`
	ExecLoginCancelled      string `json:"execLoginCancelled"`
	ExecIPLockout           string `json:"execIPLockout"`
	ExecLoginIncorrect      string `json:"execLoginIncorrect"`
	ExecNotValidated        string `json:"execNotValidated"`
	ExecMenuLoadError       string `json:"execMenuLoadError"`
	ExecMenuPasswordPrompt  string `json:"execMenuPasswordPrompt"`
	ExecPasswordAccepted    string `json:"execPasswordAccepted"`
	ExecIncorrectPassword   string `json:"execIncorrectPassword"`
	ExecTooManyAttempts     string `json:"execTooManyAttempts"`
	ExecAccessDenied        string `json:"execAccessDenied"`
	ExecLastcallTemplateErr string `json:"execLastcallTemplateErr"`
	ExecFileLoadError       string `json:"execFileLoadError"`
	ExecNoNewMail           string `json:"execNoNewMail"`
	ExecNewMailCount        string `json:"execNewMailCount"`
	ExecUserlistTemplateErr string `json:"execUserlistTemplateErr"`
	ExecPendingValidation   string `json:"execPendingValidation"`
	ExecRunCommandError     string `json:"execRunCommandError"`
	ExecRunCommandNotFound  string `json:"execRunCommandNotFound"`
	ExecRunDoorError        string `json:"execRunDoorError"`
	ExecLoginCriticalError  string `json:"execLoginCriticalError"`
	ExecVersionString       string `json:"execVersionString"`

	// Terminal size mismatch prompts (shown post-login when detected size differs from saved)
	TermSizeNewDetectedPrompt    string `json:"termSizeNewDetectedPrompt"`
	TermSizeUpdateDefaultsPrompt string `json:"termSizeUpdateDefaultsPrompt"`

	// Invisible login prompt (shown to SysOp/CoSysOp after authentication)
	InvisibleLogonPrompt string `json:"invisibleLogonPrompt"`

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
	// DOS door fields (dosemu2)
	IsDOS        bool     `json:"is_dos,omitempty"`        // true = DOS door launched via dosemu2
	DOSCommands  []string `json:"dos_commands,omitempty"`  // DOS commands to run (e.g. ["cd c:\\doors\\lord\\", "lord /n{NODE}"])
	DriveCPath   string   `json:"drive_c_path,omitempty"`  // Path to drive_c directory (default: ~/.dosemu/drive_c)
	DosemuPath   string   `json:"dosemu_path,omitempty"`   // Path to dosemu binary (default: /usr/bin/dosemu)
	DosemuConfig string   `json:"dosemu_config,omitempty"` // Path to custom .dosemurc (optional)
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

// LoginItem defines a single step in the configurable login sequence.
type LoginItem struct {
	Command     string `json:"command"`                // Required: LASTCALLS, ONELINERS, USERSTATS, NMAILSCAN, DISPLAYFILE, RUNDOOR, FASTLOGIN
	Data        string `json:"data,omitempty"`         // Optional: command-specific data (filename, script path, etc.)
	ClearScreen bool   `json:"clear_screen,omitempty"` // Optional: clear screen before this item (default false)
	PauseAfter  bool   `json:"pause_after,omitempty"`  // Optional: show pause prompt after this item (default false)
	SecLevel    int    `json:"sec_level,omitempty"`    // Optional: minimum security level required (default 0 = everyone)
}

// LoadLoginSequence loads the login sequence configuration from login.json.
// If the file is missing, returns a default sequence matching legacy behavior.
func LoadLoginSequence(configPath string) ([]LoginItem, error) {
	filePath := filepath.Join(configPath, "login.json")
	log.Printf("INFO: Loading login sequence from %s", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: login.json not found at %s, using default login sequence", filePath)
			return defaultLoginSequence(), nil
		}
		return nil, fmt.Errorf("failed to read login sequence file %s: %w", filePath, err)
	}

	var items []LoginItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse login sequence JSON from %s: %w", filePath, err)
	}

	// Normalize command names to uppercase
	for i := range items {
		items[i].Command = strings.ToUpper(items[i].Command)
	}

	log.Printf("INFO: Loaded login sequence with %d items", len(items))
	return items, nil
}

// defaultLoginSequence returns the built-in default login sequence matching legacy behavior.
func defaultLoginSequence() []LoginItem {
	return []LoginItem{
		{Command: "LASTCALLS"},
		{Command: "ONELINERS"},
		{Command: "USERSTATS"},
	}
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

// FTNLinkConfig defines an FTN link (uplink/downlink node).
// Echo area routing is derived from message_areas.json (areas where Network matches),
// not stored per-link. The Message Areas TUI is the canonical place to manage subscriptions.
type FTNLinkConfig struct {
	Address         string `json:"address"`                    // e.g., "21:1/100"
	PacketPassword  string `json:"packet_password"`            // Packet password (formerly "password")
	AreafixPassword string `json:"areafix_password,omitempty"` // Password for AreaFix netmail (subject line)
	Name            string `json:"name"`                       // Human-readable name
	Flavour         string `json:"flavour,omitempty"`          // Delivery flavour: Normal (default), Crash, Hold, Direct
}

// UnmarshalJSON supports backward compatibility: "password" is read into PacketPassword
// when packet_password is absent (nil pointer = field omitted vs explicitly empty string).
func (c *FTNLinkConfig) UnmarshalJSON(data []byte) error {
	var r struct {
		Address         string  `json:"address"`
		PacketPassword  *string `json:"packet_password"`
		AreafixPassword string  `json:"areafix_password,omitempty"`
		Name            string  `json:"name"`
		Flavour         string  `json:"flavour,omitempty"`
		LegacyPassword  string  `json:"password"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	c.Address = r.Address
	c.AreafixPassword = r.AreafixPassword
	c.Name = r.Name
	c.Flavour = r.Flavour
	if r.PacketPassword != nil {
		c.PacketPassword = *r.PacketPassword
	} else if r.LegacyPassword != "" {
		c.PacketPassword = r.LegacyPassword
	}
	return nil
}

// FTNNetworkConfig holds settings for a single FTN network (e.g., FSXNet, FidoNet).
// Netmail routing is derived from message_areas.json (areas where Network matches and AreaType == "netmail").
type FTNNetworkConfig struct {
	InternalTosserEnabled bool            `json:"internal_tosser_enabled"` // Enable internal tosser
	OwnAddress            string          `json:"own_address"`             // e.g., "21:4/158.1"
	PollSeconds           int             `json:"poll_interval_seconds"`   // 0 = manual only (v3mail toss/scan)
	Tearline              string          `json:"tearline,omitempty"`      // Custom tearline text for echomail
	Links                 []FTNLinkConfig `json:"links"`
}

// FTNConfig holds all FTN (FidoNet Technology Network) echomail settings.
// Loaded from configs/ftn.json.
type FTNConfig struct {
	DupeDBPath        string                      `json:"dupe_db_path"`                  // e.g., "data/ftn/dupes.json"
	InboundPath       string                      `json:"inbound_path"`                  // Where binkd deposits received bundles
	SecureInboundPath string                      `json:"secure_inbound_path,omitempty"` // Authenticated inbound
	OutboundPath      string                      `json:"outbound_path"`                 // Staging dir for outbound .PKT files
	BinkdOutboundPath string                      `json:"binkd_outbound_path"`           // Binkd outbound dir for ZIP bundles
	TempPath          string                      `json:"temp_path"`                     // Temp dir for processing
	BadAreaTag        string                      `json:"bad_area_tag,omitempty"`        // Area for unroutable messages (e.g., "BAD")
	DupeAreaTag       string                      `json:"dupe_area_tag,omitempty"`       // Area for duplicate messages (e.g., "DUPE")
	Networks          map[string]FTNNetworkConfig `json:"networks"`
}

// ServerConfig defines server-wide settings
type ServerConfig struct {
	BoardName           string `json:"boardName"`
	SysOpName           string `json:"sysOpName"`
	Timezone            string `json:"timezone,omitempty"`
	SysOpLevel          int    `json:"sysOpLevel"`
	CoSysOpLevel        int    `json:"coSysOpLevel"`
	InvisibleLevel      int    `json:"invisibleLevel"` // Access level for invisible logon prompt; 0 = use coSysOpLevel
	RegularUserLevel    int    `json:"regularUserLevel"`
	LogonLevel          int    `json:"logonLevel"`
	AnonymousLevel      int    `json:"anonymousLevel"`
	SSHPort             int    `json:"sshPort"`
	SSHHost             string `json:"sshHost"`
	SSHEnabled          bool   `json:"sshEnabled"`
	TelnetPort          int    `json:"telnetPort"`
	TelnetHost          string `json:"telnetHost"`
	TelnetEnabled       bool   `json:"telnetEnabled"`
	MaxNodes            int    `json:"maxNodes"`
	MaxConnectionsPerIP int    `json:"maxConnectionsPerIP"`
	IPBlocklistPath     string `json:"ipBlocklistPath"`
	IPAllowlistPath     string `json:"ipAllowlistPath"`
	MaxFailedLogins     int    `json:"maxFailedLogins"`
	LockoutMinutes      int    `json:"lockoutMinutes"`
	FileListingMode     string `json:"fileListingMode"`
	LegacySSHAlgorithms bool   `json:"legacySSHAlgorithms"`
	AllowNewUsers       bool   `json:"allowNewUsers"`

	// Idle timeout (0 = disabled). Applied across the entire app; any input loop
	// that calls ReadKeyWithTimeout uses this value.
	SessionIdleTimeoutMinutes int `json:"sessionIdleTimeoutMinutes"`

	// Transfer timeout in minutes for file transfers (ZModem, etc.). 0 = no timeout.
	// When exceeded, the transfer process is killed and the session returns to the BBS.
	TransferTimeoutMinutes int `json:"transferTimeoutMinutes"`

	// Number of days to retain soft-deleted user accounts before they are eligible
	// for permanent purge. 0 = purge immediately; -1 = never purge automatically.
	DeletedUserRetentionDays int `json:"deletedUserRetentionDays"`
}

// EventConfig defines a scheduled event configuration
type EventConfig struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Schedule          string            `json:"schedule"` // Cron syntax (empty for startup-only events)
	Command           string            `json:"command"`
	Args              []string          `json:"args"`
	WorkingDirectory  string            `json:"working_directory"`
	TimeoutSeconds    int               `json:"timeout_seconds"` // 0 = no timeout (for daemons)
	Enabled           bool              `json:"enabled"`
	RunAtStartup      bool              `json:"run_at_startup,omitempty"` // Launch immediately when BBS starts
	EnvironmentVars   map[string]string `json:"environment_vars,omitempty"`
	RunAfter          string            `json:"run_after,omitempty"`           // Event ID to run after
	DelayAfterSeconds int               `json:"delay_after_seconds,omitempty"` // Delay after RunAfter completes
}

// EventsConfig is the root configuration for the event scheduler
type EventsConfig struct {
	Enabled             bool          `json:"enabled"`
	MaxConcurrentEvents int           `json:"max_concurrent_events"`
	Events              []EventConfig `json:"events"`
}

// LoadServerConfig loads the server configuration from config.json
func LoadServerConfig(configPath string) (ServerConfig, error) {
	filePath := filepath.Join(configPath, "config.json")
	log.Printf("INFO: Loading server configuration from %s", filePath)

	// Default config values
	defaultConfig := ServerConfig{
		BoardName:                 "ViSiON/3 BBS",
		Timezone:                  "",
		SysOpLevel:                255,
		CoSysOpLevel:              250,
		RegularUserLevel:          10,
		LogonLevel:                100,
		AnonymousLevel:            5,
		SSHPort:                   2222,
		SSHHost:                   "0.0.0.0",
		SSHEnabled:                true,
		TelnetPort:                2323,
		TelnetHost:                "0.0.0.0",
		TelnetEnabled:             false,
		MaxNodes:                  10,
		MaxConnectionsPerIP:       3,
		MaxFailedLogins:           5,
		LockoutMinutes:            30,
		AllowNewUsers:             true,
		SessionIdleTimeoutMinutes: 5,
		TransferTimeoutMinutes:    30,
		LegacySSHAlgorithms:       true,
		DeletedUserRetentionDays:  30,
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

// LoadFTNConfig loads FTN network configuration from ftn.json.
// Returns an empty config (no networks) if the file does not exist.
func LoadFTNConfig(configPath string) (FTNConfig, error) {
	filePath := filepath.Join(configPath, "ftn.json")
	log.Printf("INFO: Loading FTN configuration from %s", filePath)

	defaultConfig := FTNConfig{
		Networks: make(map[string]FTNNetworkConfig),
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: ftn.json not found at %s. FTN disabled.", filePath)
			return defaultConfig, nil
		}
		return defaultConfig, fmt.Errorf("failed to read FTN config file %s: %w", filePath, err)
	}

	var config FTNConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("ERROR: Failed to parse FTN config JSON from %s: %v", filePath, err)
		return defaultConfig, fmt.Errorf("failed to parse FTN config JSON from %s: %w", filePath, err)
	}

	if config.Networks == nil {
		config.Networks = make(map[string]FTNNetworkConfig)
	}

	enabledCount := 0
	for name, net := range config.Networks {
		if net.InternalTosserEnabled {
			enabledCount++
			log.Printf("INFO: FTN network %q internal tosser enabled: address=%s", name, net.OwnAddress)
		}
	}
	log.Printf("INFO: Loaded FTN configuration: %d network(s), %d with internal tosser enabled", len(config.Networks), enabledCount)

	// Validate required global path fields when any network has the internal tosser enabled.
	if enabledCount > 0 {
		type requiredPath struct {
			field string
			value string
		}
		required := []requiredPath{
			{"inbound_path", config.InboundPath},
			{"outbound_path", config.OutboundPath},
			{"binkd_outbound_path", config.BinkdOutboundPath},
			{"temp_path", config.TempPath},
		}
		for _, r := range required {
			if r.value == "" {
				return defaultConfig, fmt.Errorf("ftn.json: %q is required when internal_tosser_enabled is true", r.field)
			}
		}
	}

	return config, nil
}

// SaveServerConfig writes the ServerConfig back to config.json in the given configPath directory.
func SaveServerConfig(configPath string, cfg ServerConfig) error {
	filePath := filepath.Join(configPath, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal server config: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filePath, err)
	}
	log.Printf("INFO: Server configuration saved to %s", filePath)
	return nil
}

// LoadEventsConfig loads the event scheduler configuration from events.json
func LoadEventsConfig(configPath string) (EventsConfig, error) {
	filePath := filepath.Join(configPath, "events.json")
	log.Printf("INFO: Loading event scheduler configuration from %s", filePath)

	defaultConfig := EventsConfig{
		Enabled:             false,
		MaxConcurrentEvents: 3,
		Events:              []EventConfig{},
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: events.json not found at %s. Event scheduler disabled.", filePath)
			return defaultConfig, nil
		}
		return defaultConfig, fmt.Errorf("failed to read events config file %s: %w", filePath, err)
	}

	var config EventsConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("ERROR: Failed to parse events config JSON from %s: %v", filePath, err)
		return defaultConfig, fmt.Errorf("failed to parse events config JSON from %s: %w", filePath, err)
	}

	// Set default max concurrent events if not specified
	if config.MaxConcurrentEvents <= 0 {
		config.MaxConcurrentEvents = 3
	}

	enabledCount := 0
	for _, event := range config.Events {
		if event.Enabled {
			enabledCount++
		}
	}

	log.Printf("INFO: Loaded event scheduler configuration: %d event(s), %d enabled", len(config.Events), enabledCount)

	return config, nil
}

// LoadTimezone returns a *time.Location for the given timezone string.
// It tries the value from config.json first, then the VISION3_TIMEZONE and TZ
// environment variables, falling back to time.Local if none resolve.
func LoadTimezone(configTZ string) *time.Location {
	// Try each source in order: config value, VISION3_TIMEZONE env, TZ env
	for _, tz := range []string{
		strings.TrimSpace(configTZ),
		strings.TrimSpace(os.Getenv("VISION3_TIMEZONE")),
		strings.TrimSpace(os.Getenv("TZ")),
	} {
		if tz == "" {
			continue
		}
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
		log.Printf("WARN: Invalid timezone '%s', trying next source.", tz)
	}
	return time.Local
}

// NowIn returns the current time in the configured timezone.
func NowIn(configTZ string) time.Time {
	return time.Now().In(LoadTimezone(configTZ))
}
