package menu

import (
	"fmt" // Added for errors
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"

	// Update local imports
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
)

// --- Tokenizer ---
type tokenType int

const (
	tokenCondition tokenType = iota
	tokenOperator
	tokenLParen
	tokenRParen
)

type token struct {
	typ   tokenType
	value string // Condition string or operator symbol
}

// tokenizeACS breaks the ACS string into tokens (conditions, operators, parentheses).
// This version handles operators/parentheses adjacent to conditions.
func tokenizeACS(acsString string) ([]token, error) {
	var tokens []token
	var currentCondition strings.Builder
	runes := []rune(acsString) // Work with runes for Unicode safety

	flushCondition := func() {
		if currentCondition.Len() > 0 {
			tokens = append(tokens, token{typ: tokenCondition, value: currentCondition.String()})
			currentCondition.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch r {
		case '(':
			flushCondition()
			tokens = append(tokens, token{typ: tokenLParen, value: string(r)})
		case ')':
			flushCondition()
			tokens = append(tokens, token{typ: tokenRParen, value: string(r)})
		case '&', '|', '!': // Operators are single characters
			flushCondition()
			tokens = append(tokens, token{typ: tokenOperator, value: string(r)})
		case ' ': // Whitespace acts as separator
			flushCondition()
			// skip whitespace
		default:
			// Start of a potential condition
			// Allow letters, numbers, '/', ':' within conditions
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '/' || r == ':' {
				currentCondition.WriteRune(r)
				// Continue consuming characters that are part of the condition
				// (Letters, Numbers, '/', ':')
				for i+1 < len(runes) {
					nextRune := runes[i+1]
					if (nextRune >= 'A' && nextRune <= 'Z') || (nextRune >= 'a' && nextRune <= 'z') || (nextRune >= '0' && nextRune <= '9') || nextRune == '/' || nextRune == ':' {
						currentCondition.WriteRune(nextRune)
						i++ // Consume the rune
					} else {
						break // End of condition characters
					}
				}
				flushCondition() // Flush the completed condition
			} else {
				// If it doesn't start with a letter/number/allowed symbol, but isn't whitespace or an operator/paren, it's unexpected.
				log.Printf("WARN: Ignoring unexpected character '%c' during ACS tokenization", r)
				// Consider returning an error instead? For now, just ignore.
				// return nil, fmt.Errorf("unexpected character '%c' in ACS string", r)
			}
		}
	}
	flushCondition() // Add any trailing condition

	log.Printf("DEBUG: Tokenized ACS '%s' -> %v", acsString, tokens) // Log tokens
	return tokens, nil
}

// --- Shunting-Yard ---
var precedence = map[string]int{
	"!": 3, // Unary NOT has high precedence
	"&": 2,
	"|": 1,
}

func getPrecedence(op string) int {
	if p, ok := precedence[op]; ok {
		return p
	}
	return 0
}

// infixToRPN converts infix token stream to Reverse Polish Notation using Shunting-Yard.
func infixToRPN(tokens []token) ([]token, error) {
	var outputQueue []token
	var operatorStack []token

	for _, t := range tokens {
		switch t.typ {
		case tokenCondition:
			outputQueue = append(outputQueue, t)
		case tokenOperator:
			// Handle operator precedence and associativity (assuming left-associative for & |)
			for len(operatorStack) > 0 && operatorStack[len(operatorStack)-1].typ == tokenOperator &&
				getPrecedence(operatorStack[len(operatorStack)-1].value) >= getPrecedence(t.value) {
				outputQueue = append(outputQueue, operatorStack[len(operatorStack)-1])
				operatorStack = operatorStack[:len(operatorStack)-1]
			}
			operatorStack = append(operatorStack, t)
		case tokenLParen:
			operatorStack = append(operatorStack, t)
		case tokenRParen:
			foundLParen := false
			for len(operatorStack) > 0 {
				op := operatorStack[len(operatorStack)-1]
				operatorStack = operatorStack[:len(operatorStack)-1] // Pop
				if op.typ == tokenLParen {
					foundLParen = true
					break
				}
				outputQueue = append(outputQueue, op)
			}
			if !foundLParen {
				return nil, fmt.Errorf("mismatched parentheses in ACS string")
			}
		}
	}

	// Pop remaining operators from stack to output queue
	for len(operatorStack) > 0 {
		op := operatorStack[len(operatorStack)-1]
		if op.typ == tokenLParen {
			return nil, fmt.Errorf("mismatched parentheses in ACS string")
		}
		outputQueue = append(outputQueue, op)
		operatorStack = operatorStack[:len(operatorStack)-1]
	}
	log.Printf("DEBUG: RPN Output Queue: %v", outputQueue) // Log RPN
	return outputQueue, nil
}

// --- RPN Evaluator ---
// Updated to handle unary '!' operator
func evaluateRPN(rpnQueue []token, u *user.User, s ssh.Session, terminal terminalPkg.Terminal, startTime time.Time) (bool, error) {
	var evalStack []bool

	for _, t := range rpnQueue {
		switch t.typ {
		case tokenCondition:
			// Evaluate condition (note: evaluateCondition should NO LONGER handle '!')
			result := evaluateCondition(t.value, u, s, terminal, startTime)
			evalStack = append(evalStack, result)
		case tokenOperator:
			switch t.value {
			case "!": // Unary NOT operator
				if len(evalStack) < 1 {
					return false, fmt.Errorf("invalid RPN expression: not enough operands for operator %s", t.value)
				}
				op1 := evalStack[len(evalStack)-1]
				evalStack = evalStack[:len(evalStack)-1] // Pop one
				evalStack = append(evalStack, !op1)      // Push negation
			case "&", "|": // Binary AND, OR operators
				if len(evalStack) < 2 {
					return false, fmt.Errorf("invalid RPN expression: not enough operands for operator %s", t.value)
				}
				op2 := evalStack[len(evalStack)-1]
				op1 := evalStack[len(evalStack)-2]
				evalStack = evalStack[:len(evalStack)-2] // Pop two

				var result bool
				if t.value == "&" {
					result = op1 && op2
				} else { // "|"
					result = op1 || op2
				}
				evalStack = append(evalStack, result) // Push result
			default:
				return false, fmt.Errorf("unknown operator in RPN evaluation: %s", t.value)
			}
		default:
			return false, fmt.Errorf("unexpected token type in RPN queue: %v", t.typ)
		}
	}

	if len(evalStack) != 1 {
		return false, fmt.Errorf("invalid RPN expression: final stack size is %d, expected 1", len(evalStack))
	}

	return evalStack[0], nil
}

// --- Refactored checkACS ---
// checkACS evaluates a ViSiON/2 Access Control String (ACS) against user credentials.
// Returns true if the user meets the ACS requirements, false otherwise.
func checkACS(acsString string, u *user.User, s ssh.Session, terminal terminalPkg.Terminal, startTime time.Time) bool {
	// log.Printf("DEBUG: [checkACS] Received ACS: '%s' (Length: %d)", acsString, len(acsString))

	if acsString == "" {
		// log.Printf("DEBUG: [checkACS] Empty ACS string. Allowing access.")
		return true // No ACS defined, always allow
	}

	// Handle '*' wildcard explicitly *before* tokenization
	if acsString == "*" { // <--- Check 2: Wildcard allows
		// log.Printf("DEBUG: [checkACS] Wildcard '*' matched. Allowing access.")
		return true
	}

	// Check if user is authenticated
	if u == nil {
		// log.Printf("DEBUG: [checkACS] Unauthenticated user ('%s'). Denying access.", acsString)
		return false // Unauthenticated users cannot pass non-empty, non-wildcard ACS
	}

	log.Printf("DEBUG: Checking ACS: '%s' for user '%s' (Level: %d, Flags: '%s', ID: %d, Validated: %t)", acsString, u.Handle, u.AccessLevel, u.Flags, u.ID, u.Validated)

	// 1. Tokenize
	tokens, err := tokenizeACS(acsString)
	if err != nil {
		log.Printf("ERROR: Failed to tokenize ACS string '%s': %v", acsString, err)
		return false // Fail on tokenization error
	}

	// 2. Convert to RPN
	rpnQueue, err := infixToRPN(tokens)
	if err != nil {
		log.Printf("ERROR: Failed to convert ACS string '%s' to RPN: %v", acsString, err)
		return false // Fail on RPN conversion error
	}

	// 3. Evaluate RPN
	result, err := evaluateRPN(rpnQueue, u, s, terminal, startTime)
	if err != nil {
		log.Printf("ERROR: Failed to evaluate RPN for ACS string '%s': %v", acsString, err)
		return false // Fail on evaluation error
	}

	log.Printf("DEBUG: ACS Check Result for '%s': %t", acsString, result)
	return result
}

// evaluateCondition evaluates a single ACS condition (e.g., S50, L, Fx).
// Note: It no longer handles the '!' prefix; that's done by the RPN evaluator.
// Returns true if the condition is met, false otherwise.
func evaluateCondition(condition string, u *user.User, s ssh.Session, terminal terminalPkg.Terminal, startTime time.Time) bool {
	// Negation handling removed - done in evaluateRPN

	if len(condition) == 0 {
		log.Printf("WARN: Empty condition encountered in ACS string after tokenization")
		return false // Treat empty condition as failing
	}

	code := strings.ToUpper(condition[0:1])
	value := ""
	if len(condition) > 1 {
		value = condition[1:]
	}

	var result bool
	switch code {
	case "S": // Security Level >= value
		level, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("WARN: Invalid level value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			result = u.AccessLevel >= level
		}
	case "L": // Local connection
		network := s.RemoteAddr().Network()
		addr := s.RemoteAddr().String()
		isLoopback := strings.HasPrefix(addr, "127.") || strings.HasPrefix(addr, "[::1]")
		result = (network == "pipe" || network == "unix" || isLoopback)
		log.Printf("DEBUG: ACS 'L' check: Network='%s', Addr='%s', IsLoopback=%t -> %t", network, addr, isLoopback, result)
	case "A": // ANSI graphics supported
		_, _, isPty := s.Pty()
		result = isPty
		log.Printf("DEBUG: ACS 'A' check: isPty=%t -> %t", isPty, result)
	case "F": // Flag is set
		if len(value) != 1 {
			log.Printf("WARN: Invalid flag value in ACS condition '%s': Flag must be single character", condition)
			result = false
		} else {
			result = strings.Contains(strings.ToUpper(u.Flags), strings.ToUpper(value))
		}
	case "U": // User ID == value
		id, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("WARN: Invalid user ID value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			result = u.ID == id
		}
	case "V": // User is validated
		result = u.Validated
	case "D": // File download level >= value (Using AccessLevel for now)
		level, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("WARN: Invalid level value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			// TODO: Confirm if this should be AccessLevel or a separate FileLevel
			result = u.AccessLevel >= level
		}
	case "E": // Post/Call Ratio >= value (Uploads / Calls)
		pcrThreshold, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("WARN: Invalid PCR value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			numLogons := u.TimesCalled
			if numLogons <= 0 {
				result = false
			} else {
				userPCR := (float64(u.NumUploads) / float64(numLogons)) * 100
				result = int(userPCR) >= pcrThreshold
				log.Printf("DEBUG: ACS 'E' check: Uploads=%d, Calls=%d, PCR=%f, Threshold=%d -> %t", u.NumUploads, numLogons, userPCR, pcrThreshold, result)
			}
		}
	case "H": // Current Hour == value
		hour, err := strconv.Atoi(value)
		if err != nil || hour < 0 || hour > 23 {
			log.Printf("WARN: Invalid hour value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			result = time.Now().Hour() == hour
		}
	case "P": // File Points >= value
		points, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("WARN: Invalid points value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			result = u.FilePoints >= points
		}
	case "T": // Time Left >= value (minutes)
		minutesLeftThreshold, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("WARN: Invalid time left value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			if u.TimeLimit <= 0 {
				log.Printf("DEBUG: ACS 'T' check: User has no time limit (TimeLimit=%d), passing.", u.TimeLimit)
				result = true
			} else {
				elapsedSeconds := time.Since(startTime).Seconds()
				timeLeftSeconds := float64(u.TimeLimit*60) - elapsedSeconds
				thresholdSeconds := float64(minutesLeftThreshold * 60)
				result = timeLeftSeconds >= thresholdSeconds
				log.Printf("DEBUG: ACS 'T' check: Limit=%dm, Elapsed=%.fs, Left=%.fs, Threshold=%ds -> %t", u.TimeLimit, elapsedSeconds, timeLeftSeconds, int(thresholdSeconds), result)
			}
		}
	case "W": // Day of Week == value (0=Sun, 1=Mon, ... 6=Sat)
		day, err := strconv.Atoi(value)
		if err != nil || day < 0 || day > 6 {
			log.Printf("WARN: Invalid day of week value in ACS condition '%s': %v", condition, err)
			result = false
		} else {
			result = int(time.Now().Weekday()) == day
		}
	case "Z": // String exists in PrivateNote
		result = strings.Contains(strings.ToUpper(u.PrivateNote), strings.ToUpper(value))
	case "Y": // Within Time Range hh:mm/hh:mm
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			log.Printf("WARN: Invalid time range format in ACS condition '%s': Expected hh:mm/hh:mm", condition)
			result = false
		} else {
			startTimeStr := parts[0]
			endTimeStr := parts[1]
			now := time.Now()
			layout := "15:04"
			startTime, errStart := time.Parse(layout, startTimeStr)
			endTime, errEnd := time.Parse(layout, endTimeStr)

			if errStart != nil || errEnd != nil {
				log.Printf("WARN: Invalid time format in ACS time range '%s': %v / %v", value, errStart, errEnd)
				result = false
			} else {
				startDateTime := time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), 0, 0, now.Location())
				endDateTime := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, now.Location())
				if endDateTime.Before(startDateTime) {
					result = now.After(startDateTime) || now.Before(endDateTime)
				} else {
					result = (now.After(startDateTime) || now.Equal(startDateTime)) && (now.Before(endDateTime) || now.Equal(endDateTime))
				}
				log.Printf("DEBUG: ACS 'Y' check: Range='%s' Start=%s End=%s Now=%s -> %t", value, startDateTime, endDateTime, now, result)
			}
		}
	case "B":
		log.Printf("WARN: ACS 'B' (Baud Rate) check is not supported and will always pass.")
		result = true
	case "C":
		log.Printf("WARN: ACS 'C' (Message Conference) check is not yet implemented (requires state tracking) and will always fail.")
		result = false
	case "I":
		log.Printf("WARN: ACS 'I' (Last Input) check is not implementable in the current context (check occurs before input) and will always fail.")
		result = false
	case "X":
		log.Printf("WARN: ACS 'X' (File Conference) check is not yet implemented (requires state tracking) and will always fail.")
		result = false
	default:
		log.Printf("WARN: Unknown ACS code '%s' in condition '%s'", code, condition)
		result = false
	}

	// Negation is handled by the RPN evaluator using the '!' operator
	return result
}
