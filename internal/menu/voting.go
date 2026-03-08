package menu

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// VoteTopic represents a single voting topic.
type VoteTopic struct {
	ID        int                 `json:"id"`
	Question  string              `json:"question"`
	Options   []string            `json:"options"`
	Mandatory bool                `json:"mandatory,omitempty"`
	AddLevel  int                 `json:"add_level,omitempty"` // min sec level to add choices; 0=disabled
	Votes     map[string][]string `json:"votes"`               // option index → []handle
}

// VotingData holds all voting topics.
type VotingData struct {
	Topics []VoteTopic `json:"topics"`
}

var votingMu sync.Mutex

func votingFilePath(rootConfigPath string) string {
	return filepath.Join(rootConfigPath, "..", "data", "voting.json")
}

func loadVotingData(rootConfigPath string) (*VotingData, error) {
	data, err := os.ReadFile(votingFilePath(rootConfigPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &VotingData{}, nil
		}
		return nil, fmt.Errorf("read voting.json: %w", err)
	}
	var vd VotingData
	if err := json.Unmarshal(data, &vd); err != nil {
		return nil, fmt.Errorf("parse voting.json: %w", err)
	}
	for i := range vd.Topics {
		if vd.Topics[i].Votes == nil {
			vd.Topics[i].Votes = make(map[string][]string)
		}
	}
	return &vd, nil
}

func saveVotingData(rootConfigPath string, vd *VotingData) error {
	data, err := json.MarshalIndent(vd, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal voting data: %w", err)
	}
	fp := votingFilePath(rootConfigPath)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return fmt.Errorf("create voting data dir: %w", err)
	}
	return os.WriteFile(fp, data, 0644)
}

func hasVoted(topic *VoteTopic, handle string) bool {
	lower := strings.ToLower(handle)
	for _, voters := range topic.Votes {
		for _, v := range voters {
			if strings.ToLower(v) == lower {
				return true
			}
		}
	}
	return false
}

func totalVotes(topic *VoteTopic) int {
	n := 0
	for _, v := range topic.Votes {
		n += len(v)
	}
	return n
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func wv(terminal *term.Terminal, msg string, outputMode ansi.OutputMode) {
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
}

func voteListTopics(terminal *term.Terminal, vd *VotingData, currentUser *user.User, outputMode ansi.OutputMode) {
	wv(terminal, "\r\n|15Voting Booths\r\n|08"+strings.Repeat("\xc4", 50)+"\r\n", outputMode)
	for i, t := range vd.Topics {
		tags := ""
		if t.Mandatory {
			tags += " |12[MANDATORY]|07"
		}
		if currentUser != nil && hasVoted(&t, currentUser.Handle) {
			tags += " |10[Voted]|07"
		}
		wv(terminal, fmt.Sprintf("  |11%2d|07. |15%s|07%s\r\n", i+1, t.Question, tags), outputMode)
	}
	wv(terminal, "|08"+strings.Repeat("\xc4", 50)+"\r\n", outputMode)
}

func voteListChoices(terminal *term.Terminal, topic *VoteTopic, outputMode ansi.OutputMode) {
	wv(terminal, fmt.Sprintf("\r\n|15%s\r\n|08%s\r\n", topic.Question, strings.Repeat("\xc4", 40)), outputMode)
	for i, opt := range topic.Options {
		wv(terminal, fmt.Sprintf("  |11%2d|07. |15%s|07\r\n", i+1, opt), outputMode)
	}
}

func voteShowResults(terminal *term.Terminal, topic *VoteTopic, outputMode ansi.OutputMode) {
	total := totalVotes(topic)
	wv(terminal, fmt.Sprintf("\r\n|15%s |08(%d vote%s)\r\n", topic.Question, total, pluralS(total)), outputMode)
	for i, opt := range topic.Options {
		count := len(topic.Votes[strconv.Itoa(i)])
		pct := 0.0
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		barLen := int(pct / 5)
		bar := strings.Repeat("\xdb", barLen) + strings.Repeat("\xb0", 20-barLen)
		wv(terminal, fmt.Sprintf("  |11%-20s |09%s |15%5.1f%% |08(%d)\r\n", opt, bar, pct, count), outputMode)
	}
}

// voteRecordVote atomically records a user's vote. Returns the updated topic.
func voteRecordVote(rootConfigPath string, topicIdx, optionIdx int, handle string) (*VoteTopic, error) {
	votingMu.Lock()
	defer votingMu.Unlock()
	vd, err := loadVotingData(rootConfigPath)
	if err != nil {
		return nil, err
	}
	if topicIdx < 0 || topicIdx >= len(vd.Topics) {
		return nil, fmt.Errorf("topic index %d out of range (have %d topics)", topicIdx, len(vd.Topics))
	}
	t := &vd.Topics[topicIdx]
	if optionIdx < 0 || optionIdx >= len(t.Options) {
		return nil, fmt.Errorf("option index %d out of range (have %d options)", optionIdx, len(t.Options))
	}
	if hasVoted(t, handle) {
		return t, nil
	}
	key := strconv.Itoa(optionIdx)
	t.Votes[key] = append(t.Votes[key], handle)
	return t, saveVotingData(rootConfigPath, vd)
}

// doVoteOnTopic handles the vote interaction for one topic. Returns true if the user voted.
func doVoteOnTopic(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	currentUser *user.User, vd *VotingData, topicIdx int,
	outputMode ansi.OutputMode, termWidth, termHeight int) bool {

	topic := &vd.Topics[topicIdx]
	voteListChoices(terminal, topic, outputMode)

	canAdd := topic.AddLevel > 0 && currentUser != nil && currentUser.AccessLevel >= topic.AddLevel
	prompt := fmt.Sprintf("\r\n|07Your selection |15[|111-%d|15]|07", len(topic.Options))
	if canAdd {
		prompt += ", |15[A]|07dd choice"
	}
	prompt += ": "
	wv(terminal, prompt, outputMode)

	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return false
	}
	input = strings.TrimSpace(input)

	if strings.ToUpper(input) == "A" && canAdd {
		wv(terminal, "|07New choice: ", outputMode)
		choice, err := readLineFromSessionIH(s, terminal)
		if err != nil || strings.TrimSpace(choice) == "" {
			return false
		}
		votingMu.Lock()
		fresh, loadErr := loadVotingData(e.RootConfigPath)
		if loadErr == nil {
			freshIdx := -1
			for i := range fresh.Topics {
				if fresh.Topics[i].ID == topic.ID {
					freshIdx = i
					break
				}
			}
			if freshIdx >= 0 {
				fresh.Topics[freshIdx].Options = append(fresh.Topics[freshIdx].Options, strings.TrimSpace(choice))
				if saveErr := saveVotingData(e.RootConfigPath, fresh); saveErr != nil {
					log.Printf("ERROR: Failed to save voting data after adding choice: %v", saveErr)
					votingMu.Unlock()
					wv(terminal, "|04Error saving choice.\r\n", outputMode)
					return false
				}
				*vd = *fresh
			}
		}
		votingMu.Unlock()
		wv(terminal, "|10Choice added!\r\n", outputMode)
		return false
	}

	n, err := strconv.Atoi(input)
	if err != nil || n < 1 || n > len(topic.Options) {
		wv(terminal, "\r\n|07Invalid selection. Vote not recorded.\r\n", outputMode)
		return false
	}

	updated, saveErr := voteRecordVote(e.RootConfigPath, topicIdx, n-1, currentUser.Handle)
	if saveErr != nil {
		log.Printf("ERROR: Node vote save failed: %v", saveErr)
		wv(terminal, "\r\n|04Error saving vote.\r\n", outputMode)
		return false
	}
	if updated != nil {
		vd.Topics[topicIdx] = *updated
	}
	wv(terminal, "\r\n|10Thanks for voting!\r\n\r\n", outputMode)
	voteShowResults(terminal, &vd.Topics[topicIdx], outputMode)
	e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
	return true
}

// runVoteOnMandatory forces the user to vote on any mandatory topics they haven't voted on.
// Intended for use in the login sequence via VOTEMANDATORY.
func runVoteOnMandatory(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}
	votingMu.Lock()
	vd, err := loadVotingData(e.RootConfigPath)
	votingMu.Unlock()
	if err != nil {
		return currentUser, "", nil
	}
	for i := range vd.Topics {
		if !vd.Topics[i].Mandatory || hasVoted(&vd.Topics[i], currentUser.Handle) {
			continue
		}
		wv(terminal, "\r\n|12Mandatory Voting!\r\n", outputMode)
		doVoteOnTopic(e, s, terminal, currentUser, vd, i, outputMode, termWidth, termHeight)
	}
	return currentUser, "", nil
}

// runVote presents the full voting booths interface.
func runVote(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	log.Printf("DEBUG: Node %d: Running VOTE for user %s", nodeNumber, currentUser.Handle)
	isSysOp := e.isCoSysOpOrAbove(currentUser)

	votingMu.Lock()
	vd, err := loadVotingData(e.RootConfigPath)
	votingMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading voting data.\r\n", outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		return currentUser, "", nil
	}

	if len(vd.Topics) == 0 {
		wv(terminal, "\r\n|07No voting topics right now.\r\n", outputMode)
		if isSysOp {
			wv(terminal, "|07Create first topic? [Y/N]: ", outputMode)
			input, _ := readLineFromSessionIH(s, terminal)
			if strings.ToUpper(strings.TrimSpace(input)) == "Y" {
				vd = voteAddTopic(e, s, terminal, vd, outputMode)
			}
		}
		if len(vd.Topics) == 0 {
			return currentUser, "", nil
		}
	}

	curIdx := 0
	for {
		voteListTopics(terminal, vd, currentUser, outputMode)
		topic := &vd.Topics[curIdx]
		voted := hasVoted(topic, currentUser.Handle)

		wv(terminal, fmt.Sprintf("\r\n|07Current topic |15[|11%d|15]|07: |15%s|07\r\n", curIdx+1, topic.Question), outputMode)

		prompt := "\r\n|15[V]|07ote  |15[L]|07ist choices  "
		if voted {
			prompt += "|15[R]|07esults  "
		}
		prompt += "|15[N]|07ext  |15[S]|07elect #  "
		if isSysOp {
			prompt += "|15[A]|07dd topic  |15[D]|07el topic  "
		}
		prompt += "|15[Q]|07uit: "
		wv(terminal, prompt, outputMode)

		input, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			return currentUser, "", nil
		}
		cmd := strings.ToUpper(strings.TrimSpace(input))

		switch {
		case cmd == "Q" || cmd == "":
			return currentUser, "", nil
		case cmd == "N":
			curIdx = (curIdx + 1) % len(vd.Topics)
		case cmd == "L":
			voteListChoices(terminal, topic, outputMode)
			e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		case cmd == "V":
			if voted {
				wv(terminal, "\r\n|07Sorry, can't vote twice!!\r\n", outputMode)
			} else {
				doVoteOnTopic(e, s, terminal, currentUser, vd, curIdx, outputMode, termWidth, termHeight)
			}
		case cmd == "R":
			if !voted {
				wv(terminal, "\r\n|07Sorry, you must vote first!\r\n", outputMode)
			} else {
				voteShowResults(terminal, topic, outputMode)
				e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
			}
		case cmd == "A" && isSysOp:
			vd = voteAddTopic(e, s, terminal, vd, outputMode)
		case cmd == "D" && isSysOp:
			wv(terminal, fmt.Sprintf("\r\n|07Delete topic %d (%s)? [Y/N]: ", curIdx+1, topic.Question), outputMode)
			confirm, _ := readLineFromSessionIH(s, terminal)
			if strings.ToUpper(strings.TrimSpace(confirm)) == "Y" {
				votingMu.Lock()
				fresh, loadErr := loadVotingData(e.RootConfigPath)
				if loadErr == nil {
					freshIdx := -1
					for i := range fresh.Topics {
						if fresh.Topics[i].ID == topic.ID {
							freshIdx = i
							break
						}
					}
					if freshIdx >= 0 {
						fresh.Topics = append(fresh.Topics[:freshIdx], fresh.Topics[freshIdx+1:]...)
						if saveErr := saveVotingData(e.RootConfigPath, fresh); saveErr != nil {
							log.Printf("ERROR: Failed to save voting data after topic deletion: %v", saveErr)
						} else {
							vd = fresh
							curIdx = freshIdx
						}
					}
				}
				votingMu.Unlock()
				if len(vd.Topics) == 0 {
					wv(terminal, "\r\n|07No voting topics right now.\r\n", outputMode)
					return currentUser, "", nil
				}
				if curIdx >= len(vd.Topics) {
					curIdx = 0
				}
			}
		default:
			n, err := strconv.Atoi(cmd)
			if err == nil && n >= 1 && n <= len(vd.Topics) {
				curIdx = n - 1
			}
		}
	}
}

// voteAddTopic interactively creates a new topic (sysop only).
func voteAddTopic(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	vd *VotingData, outputMode ansi.OutputMode) *VotingData {

	wv(terminal, "\r\n|07Voting question: ", outputMode)
	question, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(question) == "" {
		return vd
	}

	wv(terminal, "|07Make this topic mandatory? [Y/N]: ", outputMode)
	mandInput, _ := readLineFromSessionIH(s, terminal)
	mandatory := strings.ToUpper(strings.TrimSpace(mandInput)) == "Y"

	addLevel := 0
	wv(terminal, "|07Allow users to add their own choices? [Y/N]: ", outputMode)
	addInput, _ := readLineFromSessionIH(s, terminal)
	if strings.ToUpper(strings.TrimSpace(addInput)) == "Y" {
		wv(terminal, "|07Minimum security level to add choices: ", outputMode)
		lvlInput, _ := readLineFromSessionIH(s, terminal)
		addLevel, _ = strconv.Atoi(strings.TrimSpace(lvlInput))
	}

	t := VoteTopic{
		Question:  strings.TrimSpace(question),
		Mandatory: mandatory,
		AddLevel:  addLevel,
		Votes:     make(map[string][]string),
	}

	wv(terminal, "|07Enter choices (blank line to end):\r\n", outputMode)
	for {
		wv(terminal, fmt.Sprintf("|07Choice %d: ", len(t.Options)+1), outputMode)
		choice, err := readLineFromSessionIH(s, terminal)
		if err != nil || strings.TrimSpace(choice) == "" {
			break
		}
		t.Options = append(t.Options, strings.TrimSpace(choice))
	}

	if len(t.Options) == 0 {
		wv(terminal, "|07No choices entered, topic not created.\r\n", outputMode)
		return vd
	}

	votingMu.Lock()
	fresh, loadErr := loadVotingData(e.RootConfigPath)
	if loadErr == nil {
		// Assign ID from the reloaded data to avoid duplicates under concurrent creation.
		t.ID = len(fresh.Topics) + 1
		fresh.Topics = append(fresh.Topics, t)
		if saveErr := saveVotingData(e.RootConfigPath, fresh); saveErr != nil {
			log.Printf("ERROR: Failed to save voting data after topic creation: %v", saveErr)
		} else {
			vd = fresh
		}
	}
	votingMu.Unlock()

	wv(terminal, "|10Topic created!\r\n", outputMode)
	return vd
}
