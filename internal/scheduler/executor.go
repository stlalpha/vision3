package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/stlalpha/vision3/internal/config"
)

// executeEvent runs a scheduled event and returns the result
func (s *Scheduler) executeEvent(ctx context.Context, event config.EventConfig) EventResult {
	result := EventResult{
		EventID:   event.ID,
		StartTime: time.Now(),
	}

	log.Printf("INFO: Event '%s' (%s) started", event.ID, event.Name)

	// Build substitutions for placeholders
	substitutions := s.buildSubstitutions(event)

	// Substitute in Arguments
	substitutedArgs := make([]string, len(event.Args))
	for i, arg := range event.Args {
		newArg := arg
		for key, val := range substitutions {
			newArg = strings.ReplaceAll(newArg, key, val)
		}
		substitutedArgs[i] = newArg
	}

	// Substitute in Environment Variables
	substitutedEnv := make(map[string]string)
	if event.EnvironmentVars != nil {
		for key, val := range event.EnvironmentVars {
			newVal := val
			for subKey, subVal := range substitutions {
				newVal = strings.ReplaceAll(newVal, subKey, subVal)
			}
			substitutedEnv[key] = newVal
		}
	}

	// Create command with timeout context
	cmdCtx := ctx
	var cancel context.CancelFunc
	if event.TimeoutSeconds > 0 {
		cmdCtx, cancel = context.WithTimeout(ctx, time.Duration(event.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Substitute placeholders in command path
	substitutedCommand := event.Command
	for key, val := range substitutions {
		substitutedCommand = strings.ReplaceAll(substitutedCommand, key, val)
	}

	cmd := exec.CommandContext(cmdCtx, substitutedCommand, substitutedArgs...)

	// Set working directory if specified (with placeholder substitution)
	if event.WorkingDirectory != "" {
		workDir := event.WorkingDirectory
		for key, val := range substitutions {
			workDir = strings.ReplaceAll(workDir, key, val)
		}
		cmd.Dir = workDir
		log.Printf("DEBUG: Event '%s': setting working directory to '%s'", event.ID, cmd.Dir)
	}

	// Set environment variables
	cmd.Env = os.Environ()
	if len(substitutedEnv) > 0 {
		for key, val := range substitutedEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Add standard BBS environment variables
	cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_EVENT_ID=%s", event.ID))
	cmd.Env = append(cmd.Env, fmt.Sprintf("BBS_EVENT_NAME=%s", event.Name))

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err := cmd.Run()
	result.EndTime = time.Now()
	result.Output = stdout.String()
	result.ErrorOutput = stderr.String()

	// Determine result status
	if err != nil {
		result.Error = err
		if cmdCtx.Err() == context.DeadlineExceeded {
			result.Success = false
			result.ExitCode = -1
			log.Printf("ERROR: Event '%s' (%s) timed out after %ds", event.ID, event.Name, event.TimeoutSeconds)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Success = false
			result.ExitCode = exitErr.ExitCode()
			log.Printf("ERROR: Event '%s' (%s) failed with exit code %d", event.ID, event.Name, result.ExitCode)
			if result.ErrorOutput != "" {
				log.Printf("ERROR: Event '%s' stderr: %s", event.ID, result.ErrorOutput)
			}
		} else {
			result.Success = false
			result.ExitCode = -1
			log.Printf("ERROR: Event '%s' (%s) failed to start: %v", event.ID, event.Name, err)
		}
	} else {
		result.Success = true
		result.ExitCode = 0
		duration := result.EndTime.Sub(result.StartTime)
		log.Printf("INFO: Event '%s' (%s) completed in %.3fs (exit code: 0)", event.ID, event.Name, duration.Seconds())
		if result.Output != "" {
			log.Printf("DEBUG: Event '%s' output: %s", event.ID, result.Output)
		}
	}

	return result
}

// buildSubstitutions creates a map of placeholder substitutions for an event
func (s *Scheduler) buildSubstitutions(event config.EventConfig) map[string]string {
	now := time.Now()

	// Get BBS root directory from current working directory
	// This should be the BBS installation root where the binary is running
	bbsRoot, err := os.Getwd()
	if err != nil {
		log.Printf("WARN: Failed to get working directory: %v", err)
		bbsRoot = "."
	}

	return map[string]string{
		"{TIMESTAMP}":  strconv.FormatInt(now.Unix(), 10),
		"{EVENT_ID}":   event.ID,
		"{EVENT_NAME}": event.Name,
		"{BBS_ROOT}":   bbsRoot,
		"{DATE}":       now.Format("2006-01-02"),
		"{TIME}":       now.Format("15:04:05"),
		"{DATETIME}":   now.Format("2006-01-02 15:04:05"),
	}
}
