package ziplab

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// StepNumber maps to the original ZipLab step numbering (1-7, skip 4).
type StepNumber int

const (
	StepIntegrity  StepNumber = 1
	StepExtract    StepNumber = 2
	StepVirusScan  StepNumber = 3
	StepRemoveAds  StepNumber = 5
	StepAddComment StepNumber = 6
	StepInclude    StepNumber = 7
)

// PipelineResult holds the outcome of a ZipLab pipeline run.
type PipelineResult struct {
	Success     bool
	Description string // Extracted FILE_ID.DIZ content, if found
	StepResults []StepResult
	Error       error
}

// StepResult records the outcome of a single pipeline step.
type StepResult struct {
	Step    StepNumber
	Name    string
	Status  Status // StatusDoing, StatusPass, StatusFail
	Elapsed time.Duration
	Error   error
}

// StatusCallback is called when a step's status changes, allowing
// the caller to update the ANSI display in real time.
type StatusCallback func(step StepNumber, status Status)

// RunPipeline executes the full ZipLab processing pipeline on an archive file.
// The statusFn callback is invoked before/after each step to update the display.
// Returns a PipelineResult with the outcome and any extracted FILE_ID.DIZ.
func (p *Processor) RunPipeline(archivePath string, statusFn StatusCallback) PipelineResult {
	result := PipelineResult{Success: true}

	if statusFn == nil {
		statusFn = func(StepNumber, Status) {}
	}

	log.Printf("INFO: ZipLab pipeline starting for %s", archivePath)

	// Step 1: Test Integrity
	if p.config.Steps.TestIntegrity.Enabled {
		sr := p.runStep(StepIntegrity, "Test Integrity", statusFn, func() error {
			return p.StepTestIntegrity(archivePath)
		})
		result.StepResults = append(result.StepResults, sr)
		if sr.Error != nil {
			result.Success = false
			result.Error = fmt.Errorf("integrity test failed: %w", sr.Error)
			return result
		}
	}

	// Step 2: Extract
	var workDir string
	if p.config.Steps.ExtractToTemp.Enabled {
		var extractErr error
		sr := p.runStep(StepExtract, "Extract Archive", statusFn, func() error {
			var err error
			workDir, err = p.StepExtract(archivePath)
			extractErr = err
			return err
		})
		result.StepResults = append(result.StepResults, sr)
		if extractErr != nil {
			result.Success = false
			result.Error = fmt.Errorf("extraction failed: %w", extractErr)
			return result
		}
		// Clean up work directory when pipeline finishes
		if workDir != "" {
			defer os.RemoveAll(workDir)
		}
	}

	// Step 3: Virus Scan
	if p.config.Steps.VirusScan.Enabled && workDir != "" {
		sr := p.runStep(StepVirusScan, "Virus Scan", statusFn, func() error {
			return p.StepVirusScan(workDir)
		})
		result.StepResults = append(result.StepResults, sr)
		if sr.Error != nil {
			result.Success = false
			result.Error = fmt.Errorf("virus scan failed: %w", sr.Error)
			p.handleScanFailure(archivePath)
			return result
		}
	}

	// Step 5: FILE_ID.DIZ extraction and ad removal
	if p.config.Steps.RemoveAds.Enabled && workDir != "" {
		var diz string
		sr := p.runStep(StepRemoveAds, "FILE_ID.DIZ / Remove Ads", statusFn, func() error {
			var err error
			diz, err = p.StepRemoveAdsAndDIZ(workDir, archivePath)
			return err
		})
		result.StepResults = append(result.StepResults, sr)
		if sr.Error != nil {
			// Non-fatal — log but continue
			log.Printf("WARN: ZipLab step 5 had errors: %v", sr.Error)
		}
		if diz != "" {
			result.Description = diz
		}
	}

	// Step 6: Add comment
	if p.config.Steps.AddComment.Enabled {
		sr := p.runStep(StepAddComment, "Add Comment", statusFn, func() error {
			return p.StepAddComment(archivePath)
		})
		result.StepResults = append(result.StepResults, sr)
		if sr.Error != nil {
			// Non-fatal
			log.Printf("WARN: ZipLab step 6 had errors: %v", sr.Error)
		}
	}

	// Step 7: Include file
	if p.config.Steps.IncludeFile.Enabled {
		sr := p.runStep(StepInclude, "Include File", statusFn, func() error {
			return p.StepIncludeFile(archivePath)
		})
		result.StepResults = append(result.StepResults, sr)
		if sr.Error != nil {
			// Non-fatal
			log.Printf("WARN: ZipLab step 7 had errors: %v", sr.Error)
		}
	}

	log.Printf("INFO: ZipLab pipeline completed for %s: success=%v, description=%q",
		archivePath, result.Success, result.Description)
	return result
}

// runStep executes a single pipeline step, calling the status callback and timing it.
func (p *Processor) runStep(step StepNumber, name string, statusFn StatusCallback, fn func() error) StepResult {
	statusFn(step, StatusDoing)
	start := time.Now()

	err := fn()

	elapsed := time.Since(start)
	status := StatusPass
	if err != nil {
		status = StatusFail
	}

	statusFn(step, status)

	return StepResult{
		Step:    step,
		Name:    name,
		Status:  status,
		Elapsed: elapsed,
		Error:   err,
	}
}

// handleScanFailure handles a virus scan failure per configuration.
func (p *Processor) handleScanFailure(archivePath string) {
	switch p.config.ScanFailBehavior {
	case "quarantine":
		if p.config.QuarantinePath != "" {
			if err := os.MkdirAll(p.config.QuarantinePath, 0755); err != nil {
				log.Printf("ERROR: Failed to create quarantine directory %s: %v", p.config.QuarantinePath, err)
				if rmErr := os.Remove(archivePath); rmErr != nil {
					log.Printf("ERROR: Failed to remove infected file %s after quarantine dir failure: %v", archivePath, rmErr)
				}
				return
			}
			dest := filepath.Join(p.config.QuarantinePath, filepath.Base(archivePath))
			if err := os.Rename(archivePath, dest); err != nil {
				log.Printf("ERROR: Failed to quarantine %s: %v", archivePath, err)
				if rmErr := os.Remove(archivePath); rmErr != nil {
					log.Printf("ERROR: Failed to remove infected file %s after quarantine failure: %v", archivePath, rmErr)
				}
			} else {
				log.Printf("INFO: Quarantined infected file: %s -> %s", archivePath, dest)
			}
		} else {
			log.Printf("WARN: Quarantine path not configured, deleting %s", archivePath)
			if rmErr := os.Remove(archivePath); rmErr != nil {
				log.Printf("ERROR: Failed to remove infected file %s: %v", archivePath, rmErr)
			}
		}
	default: // "delete"
		log.Printf("INFO: Deleting infected file: %s", archivePath)
		if rmErr := os.Remove(archivePath); rmErr != nil {
			log.Printf("ERROR: Failed to remove infected file %s: %v", archivePath, rmErr)
		}
	}
}

// DisplayPipeline shows the ZIPLAB.ANS screen and runs the pipeline,
// updating NFO status indicators in real time. This is the main entry point
// for the visual ZipLab experience.
func (p *Processor) DisplayPipeline(w io.Writer, nfo *NFOConfig, ansiContent []byte, archivePath string) PipelineResult {
	// Display the ZIPLAB.ANS background
	if ansiContent != nil {
		w.Write(ansiContent)
	}

	// Build the status callback that writes ANSI sequences to the terminal
	statusFn := func(step StepNumber, status Status) {
		if nfo == nil {
			return
		}
		seq := nfo.BuildStatusSequence(int(step), status)
		if seq != "" {
			w.Write([]byte(seq))
		}
	}

	result := p.RunPipeline(archivePath, statusFn)

	// Move cursor below all NFO content so subsequent output doesn't overwrite
	if nfo != nil {
		if maxRow := nfo.MaxRow(); maxRow > 0 {
			w.Write([]byte(fmt.Sprintf("\x1b[%d;1H", maxRow+1)))
		}
	}

	return result
}
