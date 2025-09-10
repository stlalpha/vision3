package doors

import (
	"fmt"
	"regexp"
	"strings"
	"strconv"
	"time"
	"path/filepath"
	"os"
	"text/template"
	"bytes"
	"errors"
	
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/session"
)

var (
	ErrVariableNotFound    = errors.New("variable not found")
	ErrInvalidTemplate     = errors.New("invalid template")
	ErrTemplateExecution   = errors.New("template execution error")
	ErrCircularReference   = errors.New("circular reference detected")
)

// TemplateEngine handles parameter template processing and variable substitution
type TemplateEngine struct {
	variables       map[string]interface{}
	functions       template.FuncMap
	config          *TemplateConfig
	resolver        *VariableResolver
}

// TemplateConfig contains configuration for template processing
type TemplateConfig struct {
	VariablePrefix      string            `json:"variable_prefix"`      // Variable prefix (e.g., "{", "$")
	VariableSuffix      string            `json:"variable_suffix"`      // Variable suffix (e.g., "}", "")
	CaseSensitive       bool              `json:"case_sensitive"`       // Case sensitive variables
	AllowUndefined      bool              `json:"allow_undefined"`      // Allow undefined variables
	DefaultValues       map[string]string `json:"default_values"`       // Default variable values
	MaxSubstitutions    int               `json:"max_substitutions"`    // Maximum substitution depth
	StrictMode          bool              `json:"strict_mode"`          // Strict template processing
	EscapeMode          string            `json:"escape_mode"`          // Escape mode for special characters
	DateFormat          string            `json:"date_format"`          // Default date format
	TimeFormat          string            `json:"time_format"`          // Default time format
}

// VariableResolver resolves variables from various sources
type VariableResolver struct {
	sources []VariableSource
	cache   map[string]interface{}
	config  *TemplateConfig
}

// VariableSource interface for variable providers
type VariableSource interface {
	GetVariable(name string) (interface{}, bool)
	GetAllVariables() map[string]interface{}
	GetSourceName() string
	GetPriority() int
}

// BuiltinVariableSource provides built-in system variables
type BuiltinVariableSource struct {
	variables map[string]interface{}
}

// UserVariableSource provides user-related variables
type UserVariableSource struct {
	user    *user.User
	session *session.BbsSession
}

// SystemVariableSource provides system-related variables
type SystemVariableSource struct {
	systemName    string
	sysopName     string
	nodeName      string
	nodeNumber    int
	workingDir    string
	systemPath    string
	tempPath      string
}

// DoorVariableSource provides door-specific variables
type DoorVariableSource struct {
	doorConfig *DoorConfiguration
	instance   *DoorInstance
}

// EnvironmentVariableSource provides environment variables
type EnvironmentVariableSource struct {
	prefix string // Only include env vars with this prefix
}

// CustomVariableSource provides custom variables
type CustomVariableSource struct {
	variables map[string]interface{}
	name      string
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine(config *TemplateConfig) *TemplateEngine {
	if config == nil {
		config = &TemplateConfig{
			VariablePrefix:   "{",
			VariableSuffix:   "}",
			CaseSensitive:    false,
			AllowUndefined:   false,
			MaxSubstitutions: 10,
			StrictMode:       true,
			EscapeMode:       "none",
			DateFormat:       "2006-01-02",
			TimeFormat:       "15:04:05",
			DefaultValues:    make(map[string]string),
		}
	}
	
	engine := &TemplateEngine{
		variables: make(map[string]interface{}),
		config:    config,
		resolver:  NewVariableResolver(config),
	}
	
	// Initialize built-in template functions
	engine.initializeFunctions()
	
	return engine
}

// NewVariableResolver creates a new variable resolver
func NewVariableResolver(config *TemplateConfig) *VariableResolver {
	return &VariableResolver{
		sources: make([]VariableSource, 0),
		cache:   make(map[string]interface{}),
		config:  config,
	}
}

// AddVariableSource adds a variable source to the resolver
func (vr *VariableResolver) AddVariableSource(source VariableSource) {
	vr.sources = append(vr.sources, source)
	// Sort sources by priority (higher priority first)
	for i := len(vr.sources) - 1; i > 0; i-- {
		if vr.sources[i].GetPriority() > vr.sources[i-1].GetPriority() {
			vr.sources[i], vr.sources[i-1] = vr.sources[i-1], vr.sources[i]
		} else {
			break
		}
	}
}

// ProcessTemplate processes a template string with variable substitution
func (te *TemplateEngine) ProcessTemplate(templateStr string, data interface{}) (string, error) {
	// First pass: simple variable substitution
	result, err := te.substituteVariables(templateStr)
	if err != nil {
		return "", err
	}
	
	// Second pass: Go template processing if needed
	if strings.Contains(result, "{{") {
		tmpl, err := template.New("door").Funcs(te.functions).Parse(result)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrInvalidTemplate, err)
		}
		
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("%w: %v", ErrTemplateExecution, err)
		}
		
		result = buf.String()
	}
	
	return result, nil
}

// ProcessArguments processes command line arguments with variable substitution
func (te *TemplateEngine) ProcessArguments(args []string, data interface{}) ([]string, error) {
	result := make([]string, len(args))
	
	for i, arg := range args {
		processed, err := te.ProcessTemplate(arg, data)
		if err != nil {
			return nil, fmt.Errorf("error processing argument %d '%s': %w", i, arg, err)
		}
		result[i] = processed
	}
	
	return result, nil
}

// ProcessEnvironment processes environment variables with substitution
func (te *TemplateEngine) ProcessEnvironment(env map[string]string, data interface{}) (map[string]string, error) {
	result := make(map[string]string)
	
	for key, value := range env {
		processedKey, err := te.ProcessTemplate(key, data)
		if err != nil {
			return nil, fmt.Errorf("error processing environment key '%s': %w", key, err)
		}
		
		processedValue, err := te.ProcessTemplate(value, data)
		if err != nil {
			return nil, fmt.Errorf("error processing environment value '%s': %w", value, err)
		}
		
		result[processedKey] = processedValue
	}
	
	return result, nil
}

// substituteVariables performs variable substitution
func (te *TemplateEngine) substituteVariables(input string) (string, error) {
	if input == "" {
		return input, nil
	}
	
	// Create regex pattern for variables
	pattern := regexp.QuoteMeta(te.config.VariablePrefix) + `([^` + regexp.QuoteMeta(te.config.VariableSuffix) + `]+)` + regexp.QuoteMeta(te.config.VariableSuffix)
	re := regexp.MustCompile(pattern)
	
	result := input
	substitutions := 0
	processed := make(map[string]bool) // Prevent circular references
	
	for substitutions < te.config.MaxSubstitutions {
		matches := re.FindAllStringSubmatch(result, -1)
		if len(matches) == 0 {
			break // No more variables to substitute
		}
		
		madeSubstitution := false
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			
			varName := match[1]
			placeholder := match[0]
			
			// Check for circular references
			if processed[varName] {
				return "", fmt.Errorf("%w: variable '%s'", ErrCircularReference, varName)
			}
			
			// Get variable value
			value, err := te.getVariableValue(varName)
			if err != nil {
				if te.config.AllowUndefined {
					continue // Skip undefined variables
				}
				return "", err
			}
			
			// Convert value to string
			strValue := te.convertValueToString(value)
			
			// Perform substitution
			result = strings.ReplaceAll(result, placeholder, strValue)
			processed[varName] = true
			madeSubstitution = true
		}
		
		if !madeSubstitution {
			break // No substitutions made, avoid infinite loop
		}
		
		substitutions++
	}
	
	if substitutions >= te.config.MaxSubstitutions {
		return "", errors.New("maximum substitution depth exceeded")
	}
	
	return result, nil
}

// getVariableValue retrieves a variable value from the resolver
func (te *TemplateEngine) getVariableValue(name string) (interface{}, error) {
	// Handle case sensitivity
	if !te.config.CaseSensitive {
		name = strings.ToLower(name)
	}
	
	// Check direct variables first
	if value, exists := te.variables[name]; exists {
		return value, nil
	}
	
	// Use resolver to find variable
	value, found := te.resolver.GetVariable(name)
	if !found {
		// Check default values
		if defaultValue, exists := te.config.DefaultValues[name]; exists {
			return defaultValue, nil
		}
		
		return nil, fmt.Errorf("%w: %s", ErrVariableNotFound, name)
	}
	
	return value, nil
}

// convertValueToString converts any value to string
func (te *TemplateEngine) convertValueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case time.Time:
		return v.Format(te.config.TimeFormat)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// initializeFunctions sets up built-in template functions
func (te *TemplateEngine) initializeFunctions() {
	te.functions = template.FuncMap{
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"title":     strings.Title,
		"trim":      strings.TrimSpace,
		"replace":   strings.ReplaceAll,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"split":     strings.Split,
		"join":      strings.Join,
		"padLeft":   te.padLeft,
		"padRight":  te.padRight,
		"substring": te.substring,
		"length":    te.length,
		"default":   te.defaultValue,
		"formatDate": te.formatDate,
		"formatTime": te.formatTime,
		"now":       time.Now,
		"env":       os.Getenv,
		"basename":  filepath.Base,
		"dirname":   filepath.Dir,
		"extname":   filepath.Ext,
		"abs":       filepath.Abs,
		"clean":     filepath.Clean,
		"exists":    te.fileExists,
		"isDir":     te.isDirectory,
		"add":       te.add,
		"sub":       te.sub,
		"mul":       te.mul,
		"div":       te.div,
		"mod":       te.mod,
		"eq":        te.eq,
		"ne":        te.ne,
		"lt":        te.lt,
		"le":        te.le,
		"gt":        te.gt,
		"ge":        te.ge,
		"and":       te.and,
		"or":        te.or,
		"not":       te.not,
	}
}

// Template function implementations
func (te *TemplateEngine) padLeft(s string, width int, pad string) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(pad, width-len(s)) + s
}

func (te *TemplateEngine) padRight(s string, width int, pad string) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(pad, width-len(s))
}

func (te *TemplateEngine) substring(s string, start, length int) string {
	if start < 0 || start >= len(s) {
		return ""
	}
	end := start + length
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

func (te *TemplateEngine) length(v interface{}) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case []interface{}:
		return len(val)
	case map[string]interface{}:
		return len(val)
	default:
		return 0
	}
}

func (te *TemplateEngine) defaultValue(value, defaultVal interface{}) interface{} {
	if value == nil || value == "" {
		return defaultVal
	}
	return value
}

func (te *TemplateEngine) formatDate(t time.Time, format string) string {
	if format == "" {
		format = te.config.DateFormat
	}
	return t.Format(format)
}

func (te *TemplateEngine) formatTime(t time.Time, format string) string {
	if format == "" {
		format = te.config.TimeFormat
	}
	return t.Format(format)
}

func (te *TemplateEngine) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (te *TemplateEngine) isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Math functions
func (te *TemplateEngine) add(a, b interface{}) interface{} {
	return te.mathOp(a, b, func(x, y float64) float64 { return x + y })
}

func (te *TemplateEngine) sub(a, b interface{}) interface{} {
	return te.mathOp(a, b, func(x, y float64) float64 { return x - y })
}

func (te *TemplateEngine) mul(a, b interface{}) interface{} {
	return te.mathOp(a, b, func(x, y float64) float64 { return x * y })
}

func (te *TemplateEngine) div(a, b interface{}) interface{} {
	return te.mathOp(a, b, func(x, y float64) float64 { return x / y })
}

func (te *TemplateEngine) mod(a, b interface{}) interface{} {
	return te.mathOp(a, b, func(x, y float64) float64 { return float64(int(x) % int(y)) })
}

func (te *TemplateEngine) mathOp(a, b interface{}, op func(float64, float64) float64) interface{} {
	va := te.toFloat64(a)
	vb := te.toFloat64(b)
	return op(va, vb)
}

func (te *TemplateEngine) toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

// Comparison functions
func (te *TemplateEngine) eq(a, b interface{}) bool { return a == b }
func (te *TemplateEngine) ne(a, b interface{}) bool { return a != b }
func (te *TemplateEngine) lt(a, b interface{}) bool {
	return te.toFloat64(a) < te.toFloat64(b)
}
func (te *TemplateEngine) le(a, b interface{}) bool {
	return te.toFloat64(a) <= te.toFloat64(b)
}
func (te *TemplateEngine) gt(a, b interface{}) bool {
	return te.toFloat64(a) > te.toFloat64(b)
}
func (te *TemplateEngine) ge(a, b interface{}) bool {
	return te.toFloat64(a) >= te.toFloat64(b)
}

// Logical functions
func (te *TemplateEngine) and(a, b interface{}) bool {
	return te.toBool(a) && te.toBool(b)
}
func (te *TemplateEngine) or(a, b interface{}) bool {
	return te.toBool(a) || te.toBool(b)
}
func (te *TemplateEngine) not(a interface{}) bool {
	return !te.toBool(a)
}

func (te *TemplateEngine) toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != "" && val != "0" && strings.ToLower(val) != "false"
	case int:
		return val != 0
	case float64:
		return val != 0
	default:
		return v != nil
	}
}

// Variable source implementations

func (bvs *BuiltinVariableSource) GetVariable(name string) (interface{}, bool) {
	value, exists := bvs.variables[name]
	return value, exists
}

func (bvs *BuiltinVariableSource) GetAllVariables() map[string]interface{} {
	return bvs.variables
}

func (bvs *BuiltinVariableSource) GetSourceName() string {
	return "builtin"
}

func (bvs *BuiltinVariableSource) GetPriority() int {
	return 1 // Lowest priority
}

func (uvs *UserVariableSource) GetVariable(name string) (interface{}, bool) {
	if uvs.user == nil {
		return nil, false
	}
	
	switch strings.ToLower(name) {
	case "userid", "user_id":
		return uvs.user.ID, true
	case "userhandle", "user_handle", "handle":
		return uvs.user.Handle, true
	case "userrealname", "user_real_name", "realname":
		return uvs.user.RealName, true
	case "userlocation", "user_location", "location":
		return uvs.user.Location, true
	case "useraccesslevel", "user_access_level", "accesslevel", "security":
		return uvs.user.AccessLevel, true
	case "usercredits", "user_credits", "credits":
		return uvs.user.Credits, true
	case "usertimeleft", "user_time_left", "timeleft":
		return uvs.user.TimeLeft, true
	case "usertimeson", "user_times_on", "timeson":
		return uvs.user.TimesOn, true
	case "userpagelength", "user_page_length", "pagelength":
		return uvs.user.PageLength, true
	case "userlastcall", "user_last_call", "lastcall":
		return uvs.user.LastCall, true
	default:
		return nil, false
	}
}

func (uvs *UserVariableSource) GetAllVariables() map[string]interface{} {
	if uvs.user == nil {
		return make(map[string]interface{})
	}
	
	return map[string]interface{}{
		"USER_ID":          uvs.user.ID,
		"USER_HANDLE":      uvs.user.Handle,
		"USER_REAL_NAME":   uvs.user.RealName,
		"USER_LOCATION":    uvs.user.Location,
		"USER_ACCESS_LEVEL": uvs.user.AccessLevel,
		"USER_CREDITS":     uvs.user.Credits,
		"USER_TIME_LEFT":   uvs.user.TimeLeft,
		"USER_TIMES_ON":    uvs.user.TimesOn,
		"USER_PAGE_LENGTH": uvs.user.PageLength,
		"USER_LAST_CALL":   uvs.user.LastCall,
	}
}

func (uvs *UserVariableSource) GetSourceName() string {
	return "user"
}

func (uvs *UserVariableSource) GetPriority() int {
	return 5
}

func (svs *SystemVariableSource) GetVariable(name string) (interface{}, bool) {
	switch strings.ToLower(name) {
	case "systemname", "system_name", "bbsname":
		return svs.systemName, true
	case "sysopname", "sysop_name":
		return svs.sysopName, true
	case "nodename", "node_name":
		return svs.nodeName, true
	case "nodenumber", "node_number", "node":
		return svs.nodeNumber, true
	case "workingdir", "working_dir", "workdir":
		return svs.workingDir, true
	case "systempath", "system_path", "syspath":
		return svs.systemPath, true
	case "temppath", "temp_path", "tmppath":
		return svs.tempPath, true
	case "currentdate", "current_date", "date":
		return time.Now().Format("2006-01-02"), true
	case "currenttime", "current_time", "time":
		return time.Now().Format("15:04:05"), true
	case "timestamp":
		return time.Now().Unix(), true
	default:
		return nil, false
	}
}

func (svs *SystemVariableSource) GetAllVariables() map[string]interface{} {
	return map[string]interface{}{
		"SYSTEM_NAME":   svs.systemName,
		"SYSOP_NAME":    svs.sysopName,
		"NODE_NAME":     svs.nodeName,
		"NODE_NUMBER":   svs.nodeNumber,
		"WORKING_DIR":   svs.workingDir,
		"SYSTEM_PATH":   svs.systemPath,
		"TEMP_PATH":     svs.tempPath,
		"CURRENT_DATE":  time.Now().Format("2006-01-02"),
		"CURRENT_TIME":  time.Now().Format("15:04:05"),
		"TIMESTAMP":     time.Now().Unix(),
	}
}

func (svs *SystemVariableSource) GetSourceName() string {
	return "system"
}

func (svs *SystemVariableSource) GetPriority() int {
	return 3
}

func (dvs *DoorVariableSource) GetVariable(name string) (interface{}, bool) {
	if dvs.doorConfig == nil {
		return nil, false
	}
	
	switch strings.ToLower(name) {
	case "doorid", "door_id":
		return dvs.doorConfig.ID, true
	case "doorname", "door_name":
		return dvs.doorConfig.Name, true
	case "doordescription", "door_description":
		return dvs.doorConfig.Description, true
	case "doorcommand", "door_command":
		return dvs.doorConfig.Command, true
	case "doorworkingdir", "door_working_dir":
		return dvs.doorConfig.WorkingDirectory, true
	case "doortimelimit", "door_time_limit":
		return dvs.doorConfig.TimeLimit, true
	case "port", "comport", "com_port":
		if dvs.instance != nil {
			return 1, true // Default COM port
		}
		return nil, false
	case "baudrate", "baud_rate":
		if dvs.instance != nil {
			return 38400, true // Default baud rate
		}
		return nil, false
	default:
		return nil, false
	}
}

func (dvs *DoorVariableSource) GetAllVariables() map[string]interface{} {
	if dvs.doorConfig == nil {
		return make(map[string]interface{})
	}
	
	vars := map[string]interface{}{
		"DOOR_ID":          dvs.doorConfig.ID,
		"DOOR_NAME":        dvs.doorConfig.Name,
		"DOOR_DESCRIPTION": dvs.doorConfig.Description,
		"DOOR_COMMAND":     dvs.doorConfig.Command,
		"DOOR_WORKING_DIR": dvs.doorConfig.WorkingDirectory,
		"DOOR_TIME_LIMIT":  dvs.doorConfig.TimeLimit,
	}
	
	if dvs.instance != nil {
		vars["PORT"] = 1
		vars["BAUD_RATE"] = 38400
	}
	
	return vars
}

func (dvs *DoorVariableSource) GetSourceName() string {
	return "door"
}

func (dvs *DoorVariableSource) GetPriority() int {
	return 4
}

func (evs *EnvironmentVariableSource) GetVariable(name string) (interface{}, bool) {
	if evs.prefix != "" && !strings.HasPrefix(name, evs.prefix) {
		return nil, false
	}
	
	value := os.Getenv(name)
	if value == "" {
		return nil, false
	}
	
	return value, true
}

func (evs *EnvironmentVariableSource) GetAllVariables() map[string]interface{} {
	vars := make(map[string]interface{})
	
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			if evs.prefix == "" || strings.HasPrefix(key, evs.prefix) {
				vars[key] = parts[1]
			}
		}
	}
	
	return vars
}

func (evs *EnvironmentVariableSource) GetSourceName() string {
	return "environment"
}

func (evs *EnvironmentVariableSource) GetPriority() int {
	return 2
}

func (cvs *CustomVariableSource) GetVariable(name string) (interface{}, bool) {
	value, exists := cvs.variables[name]
	return value, exists
}

func (cvs *CustomVariableSource) GetAllVariables() map[string]interface{} {
	return cvs.variables
}

func (cvs *CustomVariableSource) GetSourceName() string {
	return cvs.name
}

func (cvs *CustomVariableSource) GetPriority() int {
	return 10 // Highest priority for custom variables
}

// GetVariable retrieves a variable from all sources
func (vr *VariableResolver) GetVariable(name string) (interface{}, bool) {
	// Check cache first
	if value, exists := vr.cache[name]; exists {
		return value, true
	}
	
	// Handle case sensitivity
	if !vr.config.CaseSensitive {
		name = strings.ToLower(name)
	}
	
	// Search through sources in priority order
	for _, source := range vr.sources {
		if value, found := source.GetVariable(name); found {
			vr.cache[name] = value
			return value, true
		}
	}
	
	return nil, false
}

// CreateTemplateFromDoorConfig creates a complete template engine setup for a door
func CreateTemplateFromDoorConfig(doorConfig *DoorConfiguration, user *user.User, session *session.BbsSession, nodeNumber int) *TemplateEngine {
	config := &TemplateConfig{
		VariablePrefix:   "{",
		VariableSuffix:   "}",
		CaseSensitive:    false,
		AllowUndefined:   false,
		MaxSubstitutions: 10,
		StrictMode:       true,
		EscapeMode:       "none",
		DateFormat:       "2006-01-02",
		TimeFormat:       "15:04:05",
		DefaultValues:    make(map[string]string),
	}
	
	engine := NewTemplateEngine(config)
	
	// Add built-in variables
	builtins := &BuiltinVariableSource{
		variables: map[string]interface{}{
			"VERSION":     "3.0",
			"BUILD":       "beta",
			"COPYRIGHT":   "Vision/3 BBS",
		},
	}
	engine.resolver.AddVariableSource(builtins)
	
	// Add user variables
	if user != nil {
		userVars := &UserVariableSource{user: user, session: session}
		engine.resolver.AddVariableSource(userVars)
	}
	
	// Add system variables
	systemVars := &SystemVariableSource{
		systemName:  "Vision/3 BBS",
		sysopName:   "Sysop",
		nodeName:    fmt.Sprintf("Node %d", nodeNumber),
		nodeNumber:  nodeNumber,
		workingDir:  doorConfig.WorkingDirectory,
		systemPath:  "/opt/vision3",
		tempPath:    "/tmp/vision3",
	}
	engine.resolver.AddVariableSource(systemVars)
	
	// Add door variables
	doorVars := &DoorVariableSource{doorConfig: doorConfig}
	engine.resolver.AddVariableSource(doorVars)
	
	// Add environment variables
	envVars := &EnvironmentVariableSource{prefix: "BBS_"}
	engine.resolver.AddVariableSource(envVars)
	
	// Add door-specific custom variables
	if len(doorConfig.EnvironmentVariables) > 0 {
		customVars := &CustomVariableSource{
			variables: make(map[string]interface{}),
			name:      "door_custom",
		}
		for key, value := range doorConfig.EnvironmentVariables {
			customVars.variables[key] = value
		}
		engine.resolver.AddVariableSource(customVars)
	}
	
	return engine
}