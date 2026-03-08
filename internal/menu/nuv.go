package menu

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// NUVVote represents a single vote cast on a NUV candidate.
type NUVVote struct {
	Voter   string `json:"voter"`
	Yes     bool   `json:"yes"`
	Comment string `json:"comment,omitempty"`
}

// NUVCandidate represents a new user pending community voting.
// Maps to V2's NuvRec.
type NUVCandidate struct {
	Handle string    `json:"handle"`
	When   time.Time `json:"when"`
	Votes  []NUVVote `json:"votes"`
}

// NUVData holds all NUV candidates.
type NUVData struct {
	Candidates []NUVCandidate `json:"candidates"`
}

var nuvMu sync.Mutex

func nuvFilePath(rootConfigPath string) string {
	return filepath.Join(rootConfigPath, "..", "data", "nuv.json")
}

func loadNUVData(rootConfigPath string) (*NUVData, error) {
	data, err := os.ReadFile(nuvFilePath(rootConfigPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &NUVData{}, nil
		}
		return nil, fmt.Errorf("read nuv.json: %w", err)
	}
	var nd NUVData
	if err := json.Unmarshal(data, &nd); err != nil {
		return nil, fmt.Errorf("parse nuv.json: %w", err)
	}
	return &nd, nil
}

func saveNUVData(rootConfigPath string, nd *NUVData) error {
	data, err := json.MarshalIndent(nd, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal nuv data: %w", err)
	}
	return os.WriteFile(nuvFilePath(rootConfigPath), data, 0644)
}

// nuvAddCandidate adds a new user handle to the NUV queue.
// Called automatically after new user registration when AutoAddNUV is true.
func nuvAddCandidate(rootConfigPath, handle string) {
	nuvMu.Lock()
	defer nuvMu.Unlock()
	nd, err := loadNUVData(rootConfigPath)
	if err != nil {
		log.Printf("WARN: NUV: failed to load nuv.json: %v", err)
		return
	}
	lower := strings.ToLower(handle)
	for _, c := range nd.Candidates {
		if strings.ToLower(c.Handle) == lower {
			return // already queued
		}
	}
	nd.Candidates = append(nd.Candidates, NUVCandidate{
		Handle: handle,
		When:   time.Now(),
	})
	if err := saveNUVData(rootConfigPath, nd); err != nil {
		log.Printf("WARN: NUV: failed to save nuv.json: %v", err)
	}
	log.Printf("INFO: NUV: added candidate '%s' to queue", handle)
}

// nuvVoteIndex returns the index of the voter's vote in candidate.Votes, or -1.
func nuvVoteIndex(c *NUVCandidate, handle string) int {
	lower := strings.ToLower(handle)
	for i, v := range c.Votes {
		if strings.ToLower(v.Voter) == lower {
			return i
		}
	}
	return -1
}

// nuvYesCount returns the number of YES votes for a candidate.
func nuvYesCount(c *NUVCandidate) int {
	n := 0
	for _, v := range c.Votes {
		if v.Yes {
			n++
		}
	}
	return n
}

// nuvDisplayStats shows the voting stats for a candidate (clears screen first).
func nuvDisplayStats(terminal *term.Terminal, c *NUVCandidate, idx int, outputMode ansi.OutputMode) {
	yes := nuvYesCount(c)
	no := len(c.Votes) - yes
	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2J\x1b[H"), outputMode)
	wv(terminal, fmt.Sprintf("\r\n|15New User Voting — Candidate #%d\r\n|08%s\r\n", idx, strings.Repeat("\xc4", 50)), outputMode)
	wv(terminal, fmt.Sprintf("|07Voting for : |11%s\r\n", c.Handle), outputMode)
	wv(terminal, fmt.Sprintf("|07Yes votes  : |10%d\r\n", yes), outputMode)
	wv(terminal, fmt.Sprintf("|07No votes   : |12%d\r\n", no), outputMode)
	wv(terminal, fmt.Sprintf("|07Added      : |11%s\r\n", c.When.Format("01/02/2006")), outputMode)
	if len(c.Votes) > 0 {
		wv(terminal, fmt.Sprintf("\r\n|15Voter Comments:\r\n|08%s\r\n", strings.Repeat("\xc4", 40)), outputMode)
		for _, v := range c.Votes {
			vote := "|12No "
			if v.Yes {
				vote = "|10Yes"
			}
			comment := v.Comment
			if comment == "" {
				comment = "(no comment)"
			}
			wv(terminal, fmt.Sprintf("|11%-20s |07%s |07%s\r\n", v.Voter, vote, comment), outputMode)
		}
	} else {
		wv(terminal, "\r\n|07No votes yet.\r\n", outputMode)
	}
	wv(terminal, "\r\n", outputMode)
}

// nuvApplyThresholds checks vote counts against config thresholds and acts.
// Returns true if the candidate was removed (validated or deleted).
func nuvApplyThresholds(e *MenuExecutor, nd *NUVData, idx int, userManager *user.UserMgr) bool {
	cfg := e.GetServerConfig()
	c := &nd.Candidates[idx]
	yes := nuvYesCount(c)
	no := len(c.Votes) - yes

	if cfg.NUVYesVotes > 0 && yes >= cfg.NUVYesVotes {
		shouldRemove := true
		if cfg.NUVValidate {
			shouldRemove = false
			if u, ok := userManager.GetUserByHandle(c.Handle); ok {
				u.AccessLevel = cfg.NUVLevel
				u.Validated = true
				if err := userManager.UpdateUser(u); err != nil {
					log.Printf("ERROR: NUV: failed to validate user '%s': %v", c.Handle, err)
				} else {
					log.Printf("INFO: NUV: auto-validated '%s' (level %d)", c.Handle, cfg.NUVLevel)
					shouldRemove = true
				}
			} else {
				log.Printf("ERROR: NUV: user '%s' not found during validation", c.Handle)
			}
		} else {
			log.Printf("INFO: NUV: '%s' reached YES threshold — notify SysOp to validate", c.Handle)
		}
		if shouldRemove {
			nd.Candidates = append(nd.Candidates[:idx], nd.Candidates[idx+1:]...)
			return true
		}
		return false
	}

	if cfg.NUVNoVotes > 0 && no >= cfg.NUVNoVotes {
		if cfg.NUVKill {
			if u, ok := userManager.GetUserByHandle(c.Handle); ok {
				u.DeletedUser = true
				if err := userManager.UpdateUser(u); err != nil {
					log.Printf("ERROR: NUV: failed to delete user '%s': %v", c.Handle, err)
				} else {
					log.Printf("INFO: NUV: auto-deleted '%s' (voted off)", c.Handle)
				}
			}
		} else {
			log.Printf("INFO: NUV: '%s' reached NO threshold — notify SysOp to delete", c.Handle)
		}
		nd.Candidates = append(nd.Candidates[:idx], nd.Candidates[idx+1:]...)
		return true
	}

	return false
}

// nuvVoteOn handles the interactive vote on a single candidate.
// Returns true if the candidate was removed from the queue after threshold.
func nuvVoteOn(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nd *NUVData, idx int, outputMode ansi.OutputMode,
	termWidth, termHeight int) bool {

	ih := getSessionIH(s)
	c := &nd.Candidates[idx]
	nuvDisplayStats(terminal, c, idx+1, outputMode)

	voterIdx := nuvVoteIndex(c, currentUser.Handle)
	if voterIdx >= 0 {
		vote := "|12No"
		if c.Votes[voterIdx].Yes {
			vote = "|10Yes"
		}
		wv(terminal, fmt.Sprintf("|07Your current vote: %s\r\n\r\n", vote), outputMode)
	}

	wv(terminal, "|15[Y]|07es  |15[N]|07o  |15[C]|07omment  |15[R]|07eshow  |15[Q]|07uit: ", outputMode)

	for {
		key, err := ih.ReadKey()
		if err != nil {
			return false
		}
		switch {
		case key == 'Q' || key == 'q' || key == editor.KeyEsc:
			return false
		case key == 'R' || key == 'r':
			nuvDisplayStats(terminal, c, idx+1, outputMode)
			if voterIdx >= 0 {
				vote := "|12No"
				if c.Votes[voterIdx].Yes {
					vote = "|10Yes"
				}
				wv(terminal, fmt.Sprintf("|07Your current vote: %s\r\n\r\n", vote), outputMode)
			}
			wv(terminal, "|15[Y]|07es  |15[N]|07o  |15[C]|07omment  |15[R]|07eshow  |15[Q]|07uit: ", outputMode)
		case key == 'C' || key == 'c':
			if voterIdx < 0 {
				wv(terminal, "\r\n|07You must vote before adding a comment.\r\n", outputMode)
				wv(terminal, "|15[Y]|07es  |15[N]|07o  |15[C]|07omment  |15[R]|07eshow  |15[Q]|07uit: ", outputMode)
				continue
			}
			wv(terminal, "\r\n|07Comment: ", outputMode)
			comment, _ := readLineFromSessionIH(s, terminal)
			comment = strings.TrimSpace(comment)
			nuvMu.Lock()
			fresh, loadErr := loadNUVData(e.RootConfigPath)
			if loadErr == nil {
				freshIdx := -1
				for i := range fresh.Candidates {
					if strings.EqualFold(fresh.Candidates[i].Handle, c.Handle) {
						freshIdx = i
						break
					}
				}
				if freshIdx >= 0 {
					vi := nuvVoteIndex(&fresh.Candidates[freshIdx], currentUser.Handle)
					if vi >= 0 {
						fresh.Candidates[freshIdx].Votes[vi].Comment = comment
						_ = saveNUVData(e.RootConfigPath, fresh)
						*nd = *fresh
						idx = freshIdx
						c = &nd.Candidates[idx]
					}
				}
			}
			nuvMu.Unlock()
			wv(terminal, "|10Comment saved.\r\n", outputMode)
			wv(terminal, "|15[Y]|07es  |15[N]|07o  |15[C]|07omment  |15[R]|07eshow  |15[Q]|07uit: ", outputMode)
		case key == 'Y' || key == 'y' || key == 'N' || key == 'n':
			castYes := key == 'Y' || key == 'y'
			nuvMu.Lock()
			fresh, loadErr := loadNUVData(e.RootConfigPath)
			removed := false
			if loadErr == nil {
				freshIdx := -1
				for i := range fresh.Candidates {
					if strings.EqualFold(fresh.Candidates[i].Handle, c.Handle) {
						freshIdx = i
						break
					}
				}
				if freshIdx >= 0 {
					vi := nuvVoteIndex(&fresh.Candidates[freshIdx], currentUser.Handle)
					if vi >= 0 {
						fresh.Candidates[freshIdx].Votes[vi].Yes = castYes
						if castYes {
							wv(terminal, "\r\n|10Vote changed to YES.\r\n", outputMode)
						} else {
							wv(terminal, "\r\n|12Vote changed to NO.\r\n", outputMode)
						}
					} else {
						fresh.Candidates[freshIdx].Votes = append(fresh.Candidates[freshIdx].Votes, NUVVote{
							Voter: currentUser.Handle,
							Yes:   castYes,
						})
						if castYes {
							wv(terminal, "\r\n|10YES vote cast!\r\n", outputMode)
						} else {
							wv(terminal, "\r\n|12NO vote cast!\r\n", outputMode)
						}
					}
					_ = saveNUVData(e.RootConfigPath, fresh)
					*nd = *fresh
					idx = freshIdx
					c = &nd.Candidates[idx]
					voterIdx = nuvVoteIndex(c, currentUser.Handle)
					removed = nuvApplyThresholds(e, nd, idx, userManager)
					_ = saveNUVData(e.RootConfigPath, nd)
				}
			}
			nuvMu.Unlock()
			if removed {
				wv(terminal, "|10Threshold reached — candidate processed.\r\n", outputMode)
				return true
			}
			wv(terminal, "|15[Y]|07es  |15[N]|07o  |15[C]|07omment  |15[R]|07eshow  |15[Q]|07uit: ", outputMode)
		}
	}
}
