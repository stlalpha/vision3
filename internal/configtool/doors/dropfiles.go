package doors

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"text/template"
	"bytes"
	"errors"
	
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/session"
)

var (
	ErrInvalidDropFileType = errors.New("invalid dropfile type")
	ErrTemplateError       = errors.New("template processing error")
	ErrFileCreationError   = errors.New("file creation error")
)

// DropFileGenerator generates various types of dropfiles for door games
type DropFileGenerator struct {
	templates map[DropFileType]*template.Template
	config    *DropFileConfig
}

// DropFileConfig contains configuration for dropfile generation
type DropFileConfig struct {
	OutputPath       string            `json:"output_path"`       // Base output path
	NodePath         string            `json:"node_path"`         // Node-specific path pattern
	Permissions      os.FileMode       `json:"permissions"`       // File permissions
	LineEnding       string            `json:"line_ending"`       // Line ending style
	DateFormat       string            `json:"date_format"`       // Date format
	TimeFormat       string            `json:"time_format"`       // Time format
	DefaultBaudRate  int               `json:"default_baud_rate"` // Default baud rate
	SystemName       string            `json:"system_name"`       // BBS system name
	SysopName        string            `json:"sysop_name"`        // Sysop name
	SysopPassword    string            `json:"sysop_password"`    // Sysop password
	ComPort          int               `json:"com_port"`          // COM port number
	ExtraVariables   map[string]string `json:"extra_variables"`   // Extra template variables
}

// DropFileData contains all data needed for dropfile generation
type DropFileData struct {
	// User information
	User             *user.User        `json:"user"`
	UserHandle       string            `json:"user_handle"`
	UserRealName     string            `json:"user_real_name"`
	UserLocation     string            `json:"user_location"`
	UserPassword     string            `json:"user_password"`
	UserSecurity     int               `json:"user_security"`
	UserCredits      int               `json:"user_credits"`
	UserTimeLeft     int               `json:"user_time_left"`
	UserTimeOnline   int               `json:"user_time_online"`
	UserLastCall     time.Time         `json:"user_last_call"`
	UserTimesOn      int               `json:"user_times_on"`
	UserPageLength   int               `json:"user_page_length"`
	
	// Session information
	Session          *session.BbsSession `json:"session"`
	NodeNumber       int               `json:"node_number"`
	BaudRate         int               `json:"baud_rate"`
	ConnectionType   string            `json:"connection_type"`
	CallerID         string            `json:"caller_id"`
	ConnectTime      time.Time         `json:"connect_time"`
	
	// System information
	SystemName       string            `json:"system_name"`
	SystemLocation   string            `json:"system_location"`
	SysopName        string            `json:"sysop_name"`
	SysopPassword    string            `json:"sysop_password"`
	ComPort          int               `json:"com_port"`
	
	// Door-specific information
	DoorName         string            `json:"door_name"`
	DoorPath         string            `json:"door_path"`
	TimeLimit        int               `json:"time_limit"`
	
	// Technical details
	IRQ              int               `json:"irq"`
	BaseAddress      string            `json:"base_address"`
	FossilPort       int               `json:"fossil_port"`
	TerminalType     string            `json:"terminal_type"`
	ScreenWidth      int               `json:"screen_width"`
	ScreenHeight     int               `json:"screen_height"`
	
	// Capabilities
	ANSISupport      bool              `json:"ansi_support"`
	ColorSupport     bool              `json:"color_support"`
	IBMChars         bool              `json:"ibm_chars"`
	
	// Custom fields
	CustomFields     map[string]string `json:"custom_fields"`
}

// NewDropFileGenerator creates a new dropfile generator
func NewDropFileGenerator(config *DropFileConfig) *DropFileGenerator {
	if config == nil {
		config = &DropFileConfig{
			OutputPath:      "/tmp/vision3/dropfiles",
			NodePath:        "node{NODE}",
			Permissions:     0644,
			LineEnding:      "\r\n",
			DateFormat:      "01-02-2006",
			TimeFormat:      "15:04:05",
			DefaultBaudRate: 38400,
			SystemName:      "Vision/3 BBS",
			SysopName:       "Sysop",
			SysopPassword:   "",
			ComPort:         1,
			ExtraVariables:  make(map[string]string),
		}
	}
	
	generator := &DropFileGenerator{
		templates: make(map[DropFileType]*template.Template),
		config:    config,
	}
	
	// Initialize standard templates
	generator.initializeTemplates()
	
	return generator
}

// GenerateDropFile generates a dropfile for the specified type
func (dfg *DropFileGenerator) GenerateDropFile(dropFileType DropFileType, data *DropFileData, outputPath string) (string, error) {
	if outputPath == "" {
		// Generate default path
		nodePath := strings.ReplaceAll(dfg.config.NodePath, "{NODE}", strconv.Itoa(data.NodeNumber))
		outputPath = filepath.Join(dfg.config.OutputPath, nodePath)
	}
	
	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileCreationError, err)
	}
	
	// Get filename for dropfile type
	filename := dfg.getDropFileName(dropFileType)
	fullPath := filepath.Join(outputPath, filename)
	
	// Generate content
	content, err := dfg.generateContent(dropFileType, data)
	if err != nil {
		return "", err
	}
	
	// Write file
	if err := os.WriteFile(fullPath, []byte(content), dfg.config.Permissions); err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileCreationError, err)
	}
	
	return fullPath, nil
}

// GenerateCustomDropFile generates a custom dropfile using a template
func (dfg *DropFileGenerator) GenerateCustomDropFile(config *CustomDropFileConfig, data *DropFileData, outputPath string) (string, error) {
	if config == nil {
		return "", errors.New("custom dropfile config is required")
	}
	
	// Parse custom template
	tmpl, err := template.New("custom").Parse(config.Template)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateError, err)
	}
	
	// Prepare template data
	templateData := dfg.prepareTemplateData(data)
	
	// Add custom variables
	for key, value := range config.Variables {
		templateData[key] = value
	}
	
	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateError, err)
	}
	
	// Apply line ending conversion
	content := buf.String()
	if config.LineEnding != "" {
		content = strings.ReplaceAll(content, "\n", config.LineEnding)
	}
	
	// Determine output path
	if outputPath == "" {
		nodePath := strings.ReplaceAll(dfg.config.NodePath, "{NODE}", strconv.Itoa(data.NodeNumber))
		outputPath = filepath.Join(dfg.config.OutputPath, nodePath)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileCreationError, err)
	}
	
	// Write file
	fullPath := filepath.Join(outputPath, config.FileName)
	if err := os.WriteFile(fullPath, []byte(content), dfg.config.Permissions); err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileCreationError, err)
	}
	
	return fullPath, nil
}

// CleanupDropFiles removes dropfiles for a specific node
func (dfg *DropFileGenerator) CleanupDropFiles(nodeNumber int) error {
	nodePath := strings.ReplaceAll(dfg.config.NodePath, "{NODE}", strconv.Itoa(nodeNumber))
	fullPath := filepath.Join(dfg.config.OutputPath, nodePath)
	
	// Remove the entire node directory
	return os.RemoveAll(fullPath)
}

// getDropFileName returns the filename for a dropfile type
func (dfg *DropFileGenerator) getDropFileName(dropFileType DropFileType) string {
	switch dropFileType {
	case DropFileDoorSys:
		return "DOOR.SYS"
	case DropFileChainTxt:
		return "CHAIN.TXT"
	case DropFileDorinfo1:
		return "DORINFO1.DEF"
	case DropFileCallinfo:
		return "CALLINFO.BBS"
	case DropFileUserinfo:
		return "USERINFO.DAT"
	case DropFileModinfo:
		return "MODINFO.DAT"
	default:
		return "DROPFILE.DAT"
	}
}

// generateContent generates the content for a specific dropfile type
func (dfg *DropFileGenerator) generateContent(dropFileType DropFileType, data *DropFileData) (string, error) {
	tmpl, exists := dfg.templates[dropFileType]
	if !exists {
		return "", ErrInvalidDropFileType
	}
	
	// Prepare template data
	templateData := dfg.prepareTemplateData(data)
	
	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateError, err)
	}
	
	return buf.String(), nil
}

// prepareTemplateData prepares data for template execution
func (dfg *DropFileGenerator) prepareTemplateData(data *DropFileData) map[string]interface{} {
	templateData := map[string]interface{}{
		// User data
		"UserHandle":     data.UserHandle,
		"UserRealName":   data.UserRealName,
		"UserLocation":   data.UserLocation,
		"UserPassword":   data.UserPassword,
		"UserSecurity":   data.UserSecurity,
		"UserCredits":    data.UserCredits,
		"UserTimeLeft":   data.UserTimeLeft,
		"UserTimeOnline": data.UserTimeOnline,
		"UserTimesOn":    data.UserTimesOn,
		"UserPageLength": data.UserPageLength,
		
		// Session data
		"NodeNumber":      data.NodeNumber,
		"BaudRate":        data.BaudRate,
		"ConnectionType":  data.ConnectionType,
		"CallerID":        data.CallerID,
		"ConnectTime":     data.ConnectTime.Format(dfg.config.TimeFormat),
		"ConnectDate":     data.ConnectTime.Format(dfg.config.DateFormat),
		
		// System data
		"SystemName":     data.SystemName,
		"SystemLocation": data.SystemLocation,
		"SysopName":      data.SysopName,
		"SysopPassword":  data.SysopPassword,
		"ComPort":        data.ComPort,
		
		// Door data
		"DoorName":   data.DoorName,
		"DoorPath":   data.DoorPath,
		"TimeLimit":  data.TimeLimit,
		
		// Technical data
		"IRQ":          data.IRQ,
		"BaseAddress":  data.BaseAddress,
		"FossilPort":   data.FossilPort,
		"TerminalType": data.TerminalType,
		"ScreenWidth":  data.ScreenWidth,
		"ScreenHeight": data.ScreenHeight,
		
		// Capabilities
		"ANSISupport":  data.ANSISupport,
		"ColorSupport": data.ColorSupport,
		"IBMChars":     data.IBMChars,
		
		// Converted values for compatibility
		"ANSIFlag":      boolToString(data.ANSISupport),
		"ColorFlag":     boolToString(data.ColorSupport),
		"IBMFlag":       boolToString(data.IBMChars),
		"GraphicsMode":  dfg.getGraphicsMode(data),
		
		// Formatted dates and times
		"LastCallDate": data.UserLastCall.Format(dfg.config.DateFormat),
		"LastCallTime": data.UserLastCall.Format(dfg.config.TimeFormat),
		"CurrentTime":  time.Now().Format(dfg.config.TimeFormat),
		"CurrentDate":  time.Now().Format(dfg.config.DateFormat),
	}
	
	// Add custom fields
	for key, value := range data.CustomFields {
		templateData[key] = value
	}
	
	// Add extra variables from config
	for key, value := range dfg.config.ExtraVariables {
		templateData[key] = value
	}
	
	return templateData
}

// initializeTemplates sets up the standard dropfile templates
func (dfg *DropFileGenerator) initializeTemplates() {
	// DOOR.SYS template
	doorSysTemplate := `{{.ComPort}}{{.LineEnding}}{{.BaudRate}}{{.LineEnding}}8{{.LineEnding}}{{.NodeNumber}}{{.LineEnding}}{{.BaudRate}}{{.LineEnding}}Y{{.LineEnding}}{{.UserRealName}}{{.LineEnding}}{{.UserLocation}}{{.LineEnding}}{{.UserPassword}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.LastCallDate}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}{{.UserTimeOnline}}{{.LineEnding}}{{.UserPageLength}}{{.LineEnding}}{{.UserHandle}}{{.LineEnding}}{{.ConnectTime}}{{.LineEnding}}{{.SystemName}}{{.LineEnding}}{{.SysopName}}{{.LineEnding}}0{{.LineEnding}}{{.ANSIFlag}}{{.LineEnding}}{{.ColorFlag}}{{.LineEnding}}{{.UserPassword}}{{.LineEnding}}{{.UserCredits}}{{.LineEnding}}{{.UserLastCall}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}{{.UserTimeOnline}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}0{{.LineEnding}}{{.ScreenWidth}}{{.LineEnding}}{{.GraphicsMode}}{{.LineEnding}}`
	
	dfg.templates[DropFileDoorSys] = template.Must(template.New("doorsys").Parse(
		strings.ReplaceAll(doorSysTemplate, "{{.LineEnding}}", dfg.config.LineEnding)))
	
	// CHAIN.TXT template
	chainTxtTemplate := `{{.UserHandle}}{{.LineEnding}}{{.UserRealName}}{{.LineEnding}}{{.UserPassword}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}{{.UserLocation}}{{.LineEnding}}{{.ANSIFlag}}{{.LineEnding}}{{.ConnectDate}}{{.LineEnding}}{{.ConnectTime}}{{.LineEnding}}{{.UserLastCall}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}{{.UserTimeOnline}}{{.LineEnding}}{{.UserPageLength}}{{.LineEnding}}{{.SystemName}}{{.LineEnding}}{{.SysopName}}{{.LineEnding}}{{.NodeNumber}}{{.LineEnding}}{{.BaudRate}}{{.LineEnding}}`
	
	dfg.templates[DropFileChainTxt] = template.Must(template.New("chaintxt").Parse(
		strings.ReplaceAll(chainTxtTemplate, "{{.LineEnding}}", dfg.config.LineEnding)))
	
	// DORINFO1.DEF template
	dorinfoTemplate := `{{.SystemName}}{{.LineEnding}}{{.SysopName}}{{.LineEnding}}{{.SysopPassword}}{{.LineEnding}}COM{{.ComPort}}{{.LineEnding}}{{.BaudRate}} BAUD,N,8,1{{.LineEnding}}0{{.LineEnding}}{{.UserHandle}}{{.LineEnding}}{{.UserRealName}}{{.LineEnding}}{{.UserLocation}}{{.LineEnding}}{{.ANSIFlag}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}`
	
	dfg.templates[DropFileDorinfo1] = template.Must(template.New("dorinfo").Parse(
		strings.ReplaceAll(dorinfoTemplate, "{{.LineEnding}}", dfg.config.LineEnding)))
	
	// CALLINFO.BBS template
	callinfoTemplate := `{{.UserHandle}}{{.LineEnding}}{{.BaudRate}}{{.LineEnding}}{{.UserLocation}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.UserPageLength}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}{{.ANSIFlag}}{{.LineEnding}}{{.ConnectDate}}{{.LineEnding}}{{.ConnectTime}}{{.LineEnding}}{{.UserRealName}}{{.LineEnding}}{{.ComPort}}{{.LineEnding}}{{.NodeNumber}}{{.LineEnding}}`
	
	dfg.templates[DropFileCallinfo] = template.Must(template.New("callinfo").Parse(
		strings.ReplaceAll(callinfoTemplate, "{{.LineEnding}}", dfg.config.LineEnding)))
	
	// USERINFO.DAT template
	userinfoTemplate := `{{.UserHandle}}{{.LineEnding}}{{.UserRealName}}{{.LineEnding}}{{.BaudRate}}{{.LineEnding}}{{.UserLocation}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.UserPageLength}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}{{.UserTimeOnline}}{{.LineEnding}}{{.ANSIFlag}}{{.LineEnding}}{{.NodeNumber}}{{.LineEnding}}{{.SystemName}}{{.LineEnding}}{{.SysopName}}{{.LineEnding}}`
	
	dfg.templates[DropFileUserinfo] = template.Must(template.New("userinfo").Parse(
		strings.ReplaceAll(userinfoTemplate, "{{.LineEnding}}", dfg.config.LineEnding)))
	
	// MODINFO.DAT template
	modinfoTemplate := `{{.UserHandle}}{{.LineEnding}}{{.SystemName}}{{.LineEnding}}{{.SysopName}}{{.LineEnding}}{{.ComPort}}{{.LineEnding}}{{.BaudRate}}{{.LineEnding}}{{.NodeNumber}}{{.LineEnding}}{{.UserRealName}}{{.LineEnding}}{{.UserLocation}}{{.LineEnding}}{{.UserSecurity}}{{.LineEnding}}{{.UserTimeLeft}}{{.LineEnding}}{{.ANSIFlag}}{{.LineEnding}}{{.UserTimesOn}}{{.LineEnding}}{{.UserPageLength}}{{.LineEnding}}`
	
	dfg.templates[DropFileModinfo] = template.Must(template.New("modinfo").Parse(
		strings.ReplaceAll(modinfoTemplate, "{{.LineEnding}}", dfg.config.LineEnding)))
}

// Helper functions

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (dfg *DropFileGenerator) getGraphicsMode(data *DropFileData) string {
	if data.ANSISupport && data.ColorSupport {
		return "2" // Full ANSI color
	} else if data.ANSISupport {
		return "1" // ANSI monochrome
	}
	return "0" // No graphics
}

// ValidateDropFileData validates dropfile data for completeness
func (dfg *DropFileGenerator) ValidateDropFileData(data *DropFileData) []string {
	var errors []string
	
	if data.UserHandle == "" {
		errors = append(errors, "User handle is required")
	}
	
	if data.UserRealName == "" {
		errors = append(errors, "User real name is required")
	}
	
	if data.NodeNumber <= 0 {
		errors = append(errors, "Valid node number is required")
	}
	
	if data.BaudRate <= 0 {
		errors = append(errors, "Valid baud rate is required")
	}
	
	if data.SystemName == "" {
		errors = append(errors, "System name is required")
	}
	
	if data.SysopName == "" {
		errors = append(errors, "Sysop name is required")
	}
	
	if data.ComPort <= 0 {
		errors = append(errors, "Valid COM port is required")
	}
	
	return errors
}

// GetSupportedDropFileTypes returns all supported dropfile types
func (dfg *DropFileGenerator) GetSupportedDropFileTypes() []DropFileType {
	return []DropFileType{
		DropFileDoorSys,
		DropFileChainTxt,
		DropFileDorinfo1,
		DropFileCallinfo,
		DropFileUserinfo,
		DropFileModinfo,
	}
}

// CreateDropFileFromSession creates dropfile data from a BBS session
func CreateDropFileFromSession(session *session.BbsSession, user *user.User, doorConfig *DoorConfiguration, nodeNumber int) *DropFileData {
	now := time.Now()
	
	data := &DropFileData{
		User:           user,
		Session:        session,
		UserHandle:     user.Handle,
		UserRealName:   user.RealName,
		UserLocation:   user.Location,
		UserPassword:   "", // Usually not included for security
		UserSecurity:   user.AccessLevel,
		UserCredits:    user.Credits,
		UserTimeLeft:   user.TimeLeft,
		UserTimeOnline: int(now.Sub(session.ConnectTime).Minutes()),
		UserLastCall:   user.LastCall,
		UserTimesOn:    user.TimesOn,
		UserPageLength: user.PageLength,
		
		NodeNumber:     nodeNumber,
		BaudRate:       session.BaudRate,
		ConnectionType: session.ConnectionType,
		CallerID:       session.CallerID,
		ConnectTime:    session.ConnectTime,
		
		DoorName:       doorConfig.Name,
		DoorPath:       doorConfig.WorkingDirectory,
		TimeLimit:      doorConfig.TimeLimit,
		
		TerminalType:   session.TerminalType,
		ScreenWidth:    session.ScreenWidth,
		ScreenHeight:   session.ScreenHeight,
		ANSISupport:    session.ANSISupport,
		ColorSupport:   session.ColorSupport,
		IBMChars:       session.IBMChars,
		
		CustomFields:   make(map[string]string),
	}
	
	// Add any custom environment variables as custom fields
	for key, value := range doorConfig.EnvironmentVariables {
		data.CustomFields[key] = value
	}
	
	return data
}