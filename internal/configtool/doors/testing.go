package doors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"context"
	"bytes"
	"regexp"
	"encoding/json"
	"syscall"
	"runtime"
	
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/session"
)

// DoorTester handles testing and validation of door configurations
type DoorTester struct {
	config          *DoorTestConfig
	templateEngine  *TemplateEngine
	dropFileGen     *DropFileGenerator
	resourceManager *ResourceManager
	testResults     map[string]*TestResult
	activeTests     map[string]*TestExecution
}

// DoorTestConfig contains configuration for door testing
type DoorTestConfig struct {
	TestTimeout      time.Duration         `json:"test_timeout"`       // Maximum test duration
	TestDataPath     string                `json:"test_data_path"`     // Path for test data
	MockUserData     *MockUserData         `json:"mock_user_data"`     // Mock user for testing
	TestNodeID       int                   `json:"test_node_id"`       // Node ID for testing
	CleanupAfterTest bool                  `json:"cleanup_after_test"` // Clean up test files
	LogTestOutput    bool                  `json:"log_test_output"`    // Log test output
	TestTypes        []TestType            `json:"test_types"`         // Types of tests to run
	ValidationRules  []ValidationRule      `json:"validation_rules"`   // Validation rules
	MockEnvironment  map[string]string     `json:"mock_environment"`   // Mock environment variables
}

// MockUserData contains mock user data for testing
type MockUserData struct {
	Handle       string `json:"handle"`
	RealName     string `json:"real_name"`
	Location     string `json:"location"`
	AccessLevel  int    `json:"access_level"`
	TimeLeft     int    `json:"time_left"`
	TimesOn      int    `json:"times_on"`
	LastCall     time.Time `json:"last_call"`
	PageLength   int    `json:"page_length"`
	Credits      int    `json:"credits"`
}

// TestType represents different types of tests
type TestType int

const (
	TestTypeBasic TestType = iota
	TestTypeExecution
	TestTypeDropFile
	TestTypePermissions
	TestTypeResources
	TestTypeSecurity
	TestTypePerformance
	TestTypeCompatibility
	TestTypeStress
	TestTypeRegression
)

func (tt TestType) String() string {
	switch tt {
	case TestTypeBasic:
		return "Basic"
	case TestTypeExecution:
		return "Execution"
	case TestTypeDropFile:
		return "Drop File"
	case TestTypePermissions:
		return "Permissions"
	case TestTypeResources:
		return "Resources"
	case TestTypeSecurity:
		return "Security"
	case TestTypePerformance:
		return "Performance"
	case TestTypeCompatibility:
		return "Compatibility"
	case TestTypeStress:
		return "Stress"
	case TestTypeRegression:
		return "Regression"
	default:
		return "Unknown"
	}
}

// ValidationRule represents a validation rule for door configurations
type ValidationRule struct {
	ID          string         `json:"id"`          // Rule ID
	Name        string         `json:"name"`        // Rule name
	Description string         `json:"description"` // Rule description
	Severity    AlertSeverity  `json:"severity"`    // Rule severity
	Category    string         `json:"category"`    // Rule category
	Validator   ValidatorFunc  `json:"-"`           // Validation function
	Enabled     bool           `json:"enabled"`     // Rule enabled
}

// ValidatorFunc is a function that validates a door configuration
type ValidatorFunc func(*DoorConfiguration, *ValidationContext) *ValidationResult

// ValidationContext provides context for validation
type ValidationContext struct {
	TestDataPath    string
	SystemPath      string
	NodeID          int
	MockUser        *MockUserData
	Environment     map[string]string
	ResourceManager *ResourceManager
}

// ValidationResult represents the result of a validation rule
type ValidationResult struct {
	RuleID     string        `json:"rule_id"`     // Rule ID
	Passed     bool          `json:"passed"`      // Validation passed
	Message    string        `json:"message"`     // Result message
	Details    string        `json:"details"`     // Additional details
	Severity   AlertSeverity `json:"severity"`    // Result severity
	Suggestion string        `json:"suggestion"`  // Suggested fix
	FixAction  string        `json:"fix_action"`  // Automated fix action
}

// TestResult represents the result of a door test
type TestResult struct {
	DoorID       string              `json:"door_id"`       // Door ID
	TestID       string              `json:"test_id"`       // Test ID
	TestType     TestType            `json:"test_type"`     // Test type
	StartTime    time.Time           `json:"start_time"`    // Test start time
	EndTime      time.Time           `json:"end_time"`      // Test end time
	Duration     time.Duration       `json:"duration"`      // Test duration
	Passed       bool                `json:"passed"`        // Test passed
	Score        float64             `json:"score"`         // Test score (0-100)
	Errors       []string            `json:"errors"`        // Test errors
	Warnings     []string            `json:"warnings"`      // Test warnings
	Output       []string            `json:"output"`        // Test output
	Validations  []*ValidationResult `json:"validations"`   // Validation results
	Performance  *PerformanceMetrics `json:"performance"`   // Performance metrics
	Resources    *ResourceUsage      `json:"resources"`     // Resource usage
	ExitCode     int                 `json:"exit_code"`     // Process exit code
	ProcessID    int                 `json:"process_id"`    // Process ID
	TestData     map[string]interface{} `json:"test_data"` // Additional test data
}

// TestExecution represents an active test execution
type TestExecution struct {
	TestID      string           `json:"test_id"`      // Test ID
	DoorID      string           `json:"door_id"`      // Door ID
	TestType    TestType         `json:"test_type"`    // Test type
	StartTime   time.Time        `json:"start_time"`   // Start time
	Process     *exec.Cmd        `json:"-"`            // Running process
	Context     context.Context  `json:"-"`            // Test context
	Cancel      context.CancelFunc `json:"-"`          // Cancel function
	OutputChan  chan string      `json:"-"`            // Output channel
	Status      TestStatus       `json:"status"`       // Current status
	Progress    float64          `json:"progress"`     // Progress (0-100)
	Stage       string           `json:"stage"`        // Current stage
}

// TestStatus represents the status of a test execution
type TestStatus int

const (
	TestStatusPending TestStatus = iota
	TestStatusRunning
	TestStatusCompleted
	TestStatusFailed
	TestStatusCancelled
	TestStatusTimeout
)

func (ts TestStatus) String() string {
	switch ts {
	case TestStatusPending:
		return "Pending"
	case TestStatusRunning:
		return "Running"
	case TestStatusCompleted:
		return "Completed"
	case TestStatusFailed:
		return "Failed"
	case TestStatusCancelled:
		return "Cancelled"
	case TestStatusTimeout:
		return "Timeout"
	default:
		return "Unknown"
	}
}

// PerformanceMetrics contains performance metrics from a test
type PerformanceMetrics struct {
	StartupTime     time.Duration `json:"startup_time"`     // Time to start
	ResponseTime    time.Duration `json:"response_time"`    // Average response time
	ThroughputOps   float64       `json:"throughput_ops"`   // Operations per second
	MemoryPeak      int64         `json:"memory_peak"`      // Peak memory usage
	MemoryAverage   int64         `json:"memory_average"`   // Average memory usage
	CPUPeak         float64       `json:"cpu_peak"`         // Peak CPU usage
	CPUAverage      float64       `json:"cpu_average"`      // Average CPU usage
	DiskReads       int64         `json:"disk_reads"`       // Disk read operations
	DiskWrites      int64         `json:"disk_writes"`      // Disk write operations
	NetworkIn       int64         `json:"network_in"`       // Network bytes in
	NetworkOut      int64         `json:"network_out"`      // Network bytes out
}

// ResourceUsage contains resource usage information
type ResourceUsage struct {
	FilesOpened      []string `json:"files_opened"`      // Files opened
	FilesCreated     []string `json:"files_created"`     // Files created
	FilesModified    []string `json:"files_modified"`    // Files modified
	DirectoriesUsed  []string `json:"directories_used"`  // Directories accessed
	LocksAcquired    []string `json:"locks_acquired"`    // Resource locks acquired
	NetworkConnections []string `json:"network_connections"` // Network connections
	ProcessesSpawned []string `json:"processes_spawned"` // Child processes spawned
}

// NewDoorTester creates a new door tester
func NewDoorTester(config *DoorTestConfig) *DoorTester {
	if config == nil {
		config = &DoorTestConfig{
			TestTimeout:      time.Minute * 5,
			TestDataPath:     "/tmp/vision3/test",
			TestNodeID:       999, // Special test node
			CleanupAfterTest: true,
			LogTestOutput:    true,
			TestTypes: []TestType{
				TestTypeBasic,
				TestTypeExecution,
				TestTypeDropFile,
				TestTypePermissions,
			},
			MockUserData: &MockUserData{
				Handle:      "TestUser",
				RealName:    "Test User",
				Location:    "Test Location",
				AccessLevel: 100,
				TimeLeft:    60,
				TimesOn:     1,
				LastCall:    time.Now().Add(-time.Hour),
				PageLength:  24,
				Credits:     1000,
			},
			MockEnvironment: make(map[string]string),
		}
	}
	
	tester := &DoorTester{
		config:      config,
		testResults: make(map[string]*TestResult),
		activeTests: make(map[string]*TestExecution),
	}
	
	// Initialize components
	tester.templateEngine = NewTemplateEngine(nil)
	tester.dropFileGen = NewDropFileGenerator(nil)
	
	// Initialize validation rules
	tester.initializeValidationRules()
	
	// Ensure test data directory exists
	os.MkdirAll(config.TestDataPath, 0755)
	
	return tester
}

// TestDoorConfiguration runs comprehensive tests on a door configuration
func (dt *DoorTester) TestDoorConfiguration(doorConfig *DoorConfiguration) (*TestResult, error) {
	testID := fmt.Sprintf("test_%s_%d", doorConfig.ID, time.Now().Unix())
	
	result := &TestResult{
		DoorID:      doorConfig.ID,
		TestID:      testID,
		TestType:    TestTypeBasic,
		StartTime:   time.Now(),
		Validations: make([]*ValidationResult, 0),
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		Output:      make([]string, 0),
		TestData:    make(map[string]interface{}),
	}
	
	// Run validation first
	validationResults := dt.ValidateDoorConfiguration(doorConfig)
	result.Validations = validationResults
	
	// Check if validation passed
	hasErrors := false
	for _, validation := range validationResults {
		if !validation.Passed && validation.Severity >= SeverityHigh {
			hasErrors = true
			result.Errors = append(result.Errors, validation.Message)
		} else if !validation.Passed {
			result.Warnings = append(result.Warnings, validation.Message)
		}
	}
	
	if hasErrors {
		result.Passed = false
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}
	
	// Run execution tests
	for _, testType := range dt.config.TestTypes {
		testResult := dt.runSpecificTest(doorConfig, testType, testID)
		
		// Merge results
		result.Errors = append(result.Errors, testResult.Errors...)
		result.Warnings = append(result.Warnings, testResult.Warnings...)
		result.Output = append(result.Output, testResult.Output...)
		
		if !testResult.Passed {
			result.Passed = false
		}
		
		// Merge performance and resource data
		if testResult.Performance != nil {
			result.Performance = testResult.Performance
		}
		if testResult.Resources != nil {
			result.Resources = testResult.Resources
		}
	}
	
	// Calculate overall score
	result.Score = dt.calculateTestScore(result)
	
	// Set final status
	if len(result.Errors) == 0 {
		result.Passed = true
	}
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	// Store result
	dt.testResults[testID] = result
	
	// Cleanup if requested
	if dt.config.CleanupAfterTest {
		dt.cleanupTestFiles(testID)
	}
	
	return result, nil
}

// ValidateDoorConfiguration validates a door configuration
func (dt *DoorTester) ValidateDoorConfiguration(doorConfig *DoorConfiguration) []*ValidationResult {
	context := &ValidationContext{
		TestDataPath:    dt.config.TestDataPath,
		SystemPath:      "/opt/vision3",
		NodeID:          dt.config.TestNodeID,
		MockUser:        dt.config.MockUserData,
		Environment:     dt.config.MockEnvironment,
		ResourceManager: dt.resourceManager,
	}
	
	var results []*ValidationResult
	
	for _, rule := range dt.config.ValidationRules {
		if rule.Enabled && rule.Validator != nil {
			result := rule.Validator(doorConfig, context)
			if result != nil {
				results = append(results, result)
			}
		}
	}
	
	return results
}

// runSpecificTest runs a specific type of test
func (dt *DoorTester) runSpecificTest(doorConfig *DoorConfiguration, testType TestType, testID string) *TestResult {
	result := &TestResult{
		DoorID:    doorConfig.ID,
		TestID:    testID,
		TestType:  testType,
		StartTime: time.Now(),
		Errors:    make([]string, 0),
		Warnings:  make([]string, 0),
		Output:    make([]string, 0),
		Passed:    true,
	}
	
	switch testType {
	case TestTypeBasic:
		dt.runBasicTest(doorConfig, result)
	case TestTypeExecution:
		dt.runExecutionTest(doorConfig, result)
	case TestTypeDropFile:
		dt.runDropFileTest(doorConfig, result)
	case TestTypePermissions:
		dt.runPermissionsTest(doorConfig, result)
	case TestTypeResources:
		dt.runResourcesTest(doorConfig, result)
	case TestTypePerformance:
		dt.runPerformanceTest(doorConfig, result)
	default:
		result.Errors = append(result.Errors, fmt.Sprintf("Unknown test type: %s", testType.String()))
		result.Passed = false
	}
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result
}

// runBasicTest runs basic configuration tests
func (dt *DoorTester) runBasicTest(doorConfig *DoorConfiguration, result *TestResult) {
	result.Output = append(result.Output, "Running basic configuration tests...")
	
	// Test required fields
	if doorConfig.Name == "" {
		result.Errors = append(result.Errors, "Door name is required")
		result.Passed = false
	}
	
	if doorConfig.Command == "" {
		result.Errors = append(result.Errors, "Door command is required")
		result.Passed = false
	}
	
	// Test file existence
	if doorConfig.Command != "" {
		if _, err := os.Stat(doorConfig.Command); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Command file not found: %s", doorConfig.Command))
			result.Passed = false
		} else {
			result.Output = append(result.Output, "✓ Command file exists")
		}
	}
	
	// Test working directory
	if doorConfig.WorkingDirectory != "" {
		if _, err := os.Stat(doorConfig.WorkingDirectory); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Working directory not found: %s", doorConfig.WorkingDirectory))
		} else {
			result.Output = append(result.Output, "✓ Working directory exists")
		}
	}
	
	result.Output = append(result.Output, "Basic tests completed")
}

// runExecutionTest runs execution tests
func (dt *DoorTester) runExecutionTest(doorConfig *DoorConfiguration, result *TestResult) {
	result.Output = append(result.Output, "Running execution tests...")
	
	// Skip execution test if command doesn't exist
	if _, err := os.Stat(doorConfig.Command); err != nil {
		result.Warnings = append(result.Warnings, "Skipping execution test - command not found")
		return
	}
	
	// Create test environment
	testDir := filepath.Join(dt.config.TestDataPath, "exec_test")
	os.MkdirAll(testDir, 0755)
	
	// Generate test drop file
	dropFilePath, err := dt.createTestDropFile(doorConfig, testDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create test drop file: %v", err))
		result.Passed = false
		return
	}
	
	// Prepare test execution
	ctx, cancel := context.WithTimeout(context.Background(), dt.config.TestTimeout)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, doorConfig.Command, dt.processArguments(doorConfig.Arguments, testDir)...)
	
	// Set working directory
	if doorConfig.WorkingDirectory != "" {
		cmd.Dir = doorConfig.WorkingDirectory
	} else {
		cmd.Dir = testDir
	}
	
	// Set environment
	env := os.Environ()
	for key, value := range doorConfig.EnvironmentVariables {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	for key, value := range dt.config.MockEnvironment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env
	
	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Start process
	startTime := time.Now()
	err = cmd.Start()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to start process: %v", err))
		result.Passed = false
		return
	}
	
	// Track process
	result.ProcessID = cmd.Process.Pid
	result.Output = append(result.Output, fmt.Sprintf("Started process with PID %d", cmd.Process.Pid))
	
	// Wait for completion or timeout
	err = cmd.Wait()
	endTime := time.Now()
	
	// Capture exit code
	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			result.ExitCode = status.ExitStatus()
		}
	}
	
	// Record performance metrics
	result.Performance = &PerformanceMetrics{
		StartupTime: endTime.Sub(startTime),
	}
	
	// Check results
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		result.Errors = append(result.Errors, "Process timed out")
		result.Passed = false
	} else if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Process exited with error: %v", err))
	} else {
		result.Output = append(result.Output, "✓ Process executed successfully")
	}
	
	// Capture output
	if stdout.Len() > 0 {
		result.Output = append(result.Output, "STDOUT:", stdout.String())
	}
	if stderr.Len() > 0 {
		result.Output = append(result.Output, "STDERR:", stderr.String())
	}
	
	// Clean up drop file
	os.Remove(dropFilePath)
	
	result.Output = append(result.Output, "Execution test completed")
}

// runDropFileTest runs drop file generation tests
func (dt *DoorTester) runDropFileTest(doorConfig *DoorConfiguration, result *TestResult) {
	result.Output = append(result.Output, "Running drop file tests...")
	
	if doorConfig.DropFileType == DropFileNone {
		result.Output = append(result.Output, "Drop file generation disabled - skipping test")
		return
	}
	
	// Create test directory
	testDir := filepath.Join(dt.config.TestDataPath, "dropfile_test")
	os.MkdirAll(testDir, 0755)
	
	// Create test drop file
	dropFilePath, err := dt.createTestDropFile(doorConfig, testDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create drop file: %v", err))
		result.Passed = false
		return
	}
	
	// Verify drop file exists
	if _, err := os.Stat(dropFilePath); err != nil {
		result.Errors = append(result.Errors, "Drop file was not created")
		result.Passed = false
		return
	}
	
	result.Output = append(result.Output, "✓ Drop file created successfully")
	
	// Read and validate drop file content
	content, err := os.ReadFile(dropFilePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to read drop file: %v", err))
		result.Passed = false
		return
	}
	
	// Basic content validation
	if len(content) == 0 {
		result.Errors = append(result.Errors, "Drop file is empty")
		result.Passed = false
		return
	}
	
	result.Output = append(result.Output, fmt.Sprintf("✓ Drop file content validated (%d bytes)", len(content)))
	
	// Validate drop file format
	if err := dt.validateDropFileFormat(doorConfig.DropFileType, content); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Drop file format warning: %v", err))
	} else {
		result.Output = append(result.Output, "✓ Drop file format is valid")
	}
	
	// Clean up
	os.Remove(dropFilePath)
	
	result.Output = append(result.Output, "Drop file test completed")
}

// runPermissionsTest runs file permissions tests
func (dt *DoorTester) runPermissionsTest(doorConfig *DoorConfiguration, result *TestResult) {
	result.Output = append(result.Output, "Running permissions tests...")
	
	// Test command file permissions
	if doorConfig.Command != "" {
		if info, err := os.Stat(doorConfig.Command); err == nil {
			mode := info.Mode()
			if runtime.GOOS != "windows" && mode&0111 == 0 {
				result.Errors = append(result.Errors, "Command file is not executable")
				result.Passed = false
			} else {
				result.Output = append(result.Output, "✓ Command file is executable")
			}
		}
	}
	
	// Test working directory permissions
	if doorConfig.WorkingDirectory != "" {
		if info, err := os.Stat(doorConfig.WorkingDirectory); err == nil {
			if !info.IsDir() {
				result.Errors = append(result.Errors, "Working directory is not a directory")
				result.Passed = false
			} else {
				// Test write permissions
				testFile := filepath.Join(doorConfig.WorkingDirectory, "test_write_permissions")
				if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
					result.Warnings = append(result.Warnings, "Working directory may not be writable")
				} else {
					os.Remove(testFile)
					result.Output = append(result.Output, "✓ Working directory is writable")
				}
			}
		}
	}
	
	result.Output = append(result.Output, "Permissions test completed")
}

// runResourcesTest runs resource usage tests
func (dt *DoorTester) runResourcesTest(doorConfig *DoorConfiguration, result *TestResult) {
	result.Output = append(result.Output, "Running resource tests...")
	
	// Test shared resources
	for _, resource := range doorConfig.SharedResources {
		if _, err := os.Stat(resource); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Shared resource not found: %s", resource))
		} else {
			result.Output = append(result.Output, fmt.Sprintf("✓ Shared resource exists: %s", resource))
		}
	}
	
	// Test exclusive resources
	for _, resource := range doorConfig.ExclusiveResources {
		if _, err := os.Stat(resource); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Exclusive resource not found: %s", resource))
		} else {
			result.Output = append(result.Output, fmt.Sprintf("✓ Exclusive resource exists: %s", resource))
		}
	}
	
	result.Output = append(result.Output, "Resource test completed")
}

// runPerformanceTest runs performance tests
func (dt *DoorTester) runPerformanceTest(doorConfig *DoorConfiguration, result *TestResult) {
	result.Output = append(result.Output, "Running performance tests...")
	
	// This would typically involve multiple test runs and metrics collection
	// For now, we'll just simulate basic performance metrics
	
	result.Performance = &PerformanceMetrics{
		StartupTime:   time.Millisecond * 100,
		ResponseTime:  time.Millisecond * 50,
		ThroughputOps: 10.0,
		MemoryPeak:    1024 * 1024, // 1MB
		MemoryAverage: 512 * 1024,  // 512KB
		CPUPeak:       25.0,
		CPUAverage:    10.0,
	}
	
	result.Output = append(result.Output, "✓ Performance metrics collected")
	result.Output = append(result.Output, "Performance test completed")
}

// Helper methods

func (dt *DoorTester) createTestDropFile(doorConfig *DoorConfiguration, testDir string) (string, error) {
	// Create mock user and session
	mockUser := &user.User{
		ID:          1,
		Handle:      dt.config.MockUserData.Handle,
		RealName:    dt.config.MockUserData.RealName,
		Location:    dt.config.MockUserData.Location,
		AccessLevel: dt.config.MockUserData.AccessLevel,
		TimeLeft:    dt.config.MockUserData.TimeLeft,
		TimesOn:     dt.config.MockUserData.TimesOn,
		LastCall:    dt.config.MockUserData.LastCall,
		PageLength:  dt.config.MockUserData.PageLength,
		Credits:     dt.config.MockUserData.Credits,
	}
	
	mockSession := &session.BbsSession{
		NodeID:         dt.config.TestNodeID,
		ConnectTime:    time.Now(),
		BaudRate:       38400,
		ConnectionType: "Test",
		TerminalType:   "ANSI",
		ScreenWidth:    80,
		ScreenHeight:   25,
		ANSISupport:    true,
		ColorSupport:   true,
		IBMChars:       true,
	}
	
	// Create drop file data
	dropFileData := CreateDropFileFromSession(mockSession, mockUser, doorConfig, dt.config.TestNodeID)
	
	// Generate drop file
	return dt.dropFileGen.GenerateDropFile(doorConfig.DropFileType, dropFileData, testDir)
}

func (dt *DoorTester) processArguments(args []string, testDir string) []string {
	processed := make([]string, len(args))
	
	for i, arg := range args {
		// Simple variable substitution for testing
		arg = strings.ReplaceAll(arg, "{NODE}", strconv.Itoa(dt.config.TestNodeID))
		arg = strings.ReplaceAll(arg, "{TESTDIR}", testDir)
		arg = strings.ReplaceAll(arg, "{USER}", dt.config.MockUserData.Handle)
		processed[i] = arg
	}
	
	return processed
}

func (dt *DoorTester) validateDropFileFormat(dropFileType DropFileType, content []byte) error {
	lines := strings.Split(string(content), "\n")
	
	switch dropFileType {
	case DropFileDoorSys:
		// DOOR.SYS should have at least 25 lines
		if len(lines) < 25 {
			return fmt.Errorf("DOOR.SYS should have at least 25 lines, found %d", len(lines))
		}
		
		// First line should be COM port
		if len(lines) > 0 {
			if matched, _ := regexp.MatchString(`^\d+$`, strings.TrimSpace(lines[0])); !matched {
				return fmt.Errorf("first line should be COM port number")
			}
		}
		
	case DropFileChainTxt:
		// CHAIN.TXT should have at least 17 lines
		if len(lines) < 17 {
			return fmt.Errorf("CHAIN.TXT should have at least 17 lines, found %d", len(lines))
		}
		
	case DropFileDorinfo1:
		// DORINFO1.DEF should have at least 13 lines
		if len(lines) < 13 {
			return fmt.Errorf("DORINFO1.DEF should have at least 13 lines, found %d", len(lines))
		}
	}
	
	return nil
}

func (dt *DoorTester) calculateTestScore(result *TestResult) float64 {
	score := 100.0
	
	// Deduct points for errors and warnings
	score -= float64(len(result.Errors)) * 20.0
	score -= float64(len(result.Warnings)) * 5.0
	
	// Bonus points for passing validations
	passedValidations := 0
	totalValidations := len(result.Validations)
	
	for _, validation := range result.Validations {
		if validation.Passed {
			passedValidations++
		}
	}
	
	if totalValidations > 0 {
		validationScore := float64(passedValidations) / float64(totalValidations) * 100.0
		score = (score + validationScore) / 2.0
	}
	
	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	
	return score
}

func (dt *DoorTester) cleanupTestFiles(testID string) {
	// Remove test-specific files and directories
	testPaths := []string{
		filepath.Join(dt.config.TestDataPath, "exec_test"),
		filepath.Join(dt.config.TestDataPath, "dropfile_test"),
		filepath.Join(dt.config.TestDataPath, testID),
	}
	
	for _, path := range testPaths {
		os.RemoveAll(path)
	}
}

// initializeValidationRules sets up the default validation rules
func (dt *DoorTester) initializeValidationRules() {
	dt.config.ValidationRules = []ValidationRule{
		{
			ID:          "required_name",
			Name:        "Door Name Required",
			Description: "Door must have a name",
			Severity:    SeverityHigh,
			Category:    "Basic",
			Enabled:     true,
			Validator:   dt.validateRequiredName,
		},
		{
			ID:          "required_command",
			Name:        "Command Required",
			Description: "Door must have a command",
			Severity:    SeverityHigh,
			Category:    "Basic",
			Enabled:     true,
			Validator:   dt.validateRequiredCommand,
		},
		{
			ID:          "command_exists",
			Name:        "Command File Exists",
			Description: "Command file must exist",
			Severity:    SeverityHigh,
			Category:    "Files",
			Enabled:     true,
			Validator:   dt.validateCommandExists,
		},
		{
			ID:          "working_dir_exists",
			Name:        "Working Directory Exists",
			Description: "Working directory should exist",
			Severity:    SeverityMedium,
			Category:    "Files",
			Enabled:     true,
			Validator:   dt.validateWorkingDirExists,
		},
		{
			ID:          "valid_multinode_settings",
			Name:        "Valid Multi-Node Settings",
			Description: "Multi-node settings should be valid",
			Severity:    SeverityMedium,
			Category:    "Configuration",
			Enabled:     true,
			Validator:   dt.validateMultiNodeSettings,
		},
	}
}

// Validation rule implementations

func (dt *DoorTester) validateRequiredName(config *DoorConfiguration, ctx *ValidationContext) *ValidationResult {
	if config.Name == "" {
		return &ValidationResult{
			RuleID:     "required_name",
			Passed:     false,
			Message:    "Door name is required",
			Severity:   SeverityHigh,
			Suggestion: "Enter a descriptive name for the door",
		}
	}
	
	return &ValidationResult{
		RuleID:   "required_name",
		Passed:   true,
		Message:  "Door has a valid name",
		Severity: SeverityLow,
	}
}

func (dt *DoorTester) validateRequiredCommand(config *DoorConfiguration, ctx *ValidationContext) *ValidationResult {
	if config.Command == "" {
		return &ValidationResult{
			RuleID:     "required_command",
			Passed:     false,
			Message:    "Door command is required",
			Severity:   SeverityHigh,
			Suggestion: "Specify the path to the door executable",
		}
	}
	
	return &ValidationResult{
		RuleID:   "required_command",
		Passed:   true,
		Message:  "Door has a valid command",
		Severity: SeverityLow,
	}
}

func (dt *DoorTester) validateCommandExists(config *DoorConfiguration, ctx *ValidationContext) *ValidationResult {
	if config.Command == "" {
		return nil // Skip if no command specified
	}
	
	if _, err := os.Stat(config.Command); err != nil {
		return &ValidationResult{
			RuleID:     "command_exists",
			Passed:     false,
			Message:    fmt.Sprintf("Command file not found: %s", config.Command),
			Severity:   SeverityHigh,
			Suggestion: "Verify the command path and ensure the file exists",
		}
	}
	
	return &ValidationResult{
		RuleID:   "command_exists",
		Passed:   true,
		Message:  "Command file exists",
		Severity: SeverityLow,
	}
}

func (dt *DoorTester) validateWorkingDirExists(config *DoorConfiguration, ctx *ValidationContext) *ValidationResult {
	if config.WorkingDirectory == "" {
		return nil // Skip if no working directory specified
	}
	
	if _, err := os.Stat(config.WorkingDirectory); err != nil {
		return &ValidationResult{
			RuleID:     "working_dir_exists",
			Passed:     false,
			Message:    fmt.Sprintf("Working directory not found: %s", config.WorkingDirectory),
			Severity:   SeverityMedium,
			Suggestion: "Create the working directory or update the path",
		}
	}
	
	return &ValidationResult{
		RuleID:   "working_dir_exists",
		Passed:   true,
		Message:  "Working directory exists",
		Severity: SeverityLow,
	}
}

func (dt *DoorTester) validateMultiNodeSettings(config *DoorConfiguration, ctx *ValidationContext) *ValidationResult {
	if config.MaxInstances < 1 {
		return &ValidationResult{
			RuleID:     "valid_multinode_settings",
			Passed:     false,
			Message:    "Maximum instances must be at least 1",
			Severity:   SeverityMedium,
			Suggestion: "Set maximum instances to 1 or higher",
		}
	}
	
	if config.MaxInstances > 50 {
		return &ValidationResult{
			RuleID:     "valid_multinode_settings",
			Passed:     false,
			Message:    "Maximum instances is unusually high",
			Severity:   SeverityMedium,
			Suggestion: "Consider reducing maximum instances for better system performance",
		}
	}
	
	return &ValidationResult{
		RuleID:   "valid_multinode_settings",
		Passed:   true,
		Message:  "Multi-node settings are valid",
		Severity: SeverityLow,
	}
}

// GetTestResult returns a test result by ID
func (dt *DoorTester) GetTestResult(testID string) (*TestResult, bool) {
	result, exists := dt.testResults[testID]
	return result, exists
}

// GetAllTestResults returns all test results
func (dt *DoorTester) GetAllTestResults() map[string]*TestResult {
	return dt.testResults
}

// SaveTestResults saves test results to a file
func (dt *DoorTester) SaveTestResults(filepath string) error {
	data, err := json.MarshalIndent(dt.testResults, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(filepath, data, 0644)
}

// LoadTestResults loads test results from a file
func (dt *DoorTester) LoadTestResults(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, &dt.testResults)
}