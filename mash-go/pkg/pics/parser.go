package pics

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Parser parses PICS files.
type Parser struct {
	// Strict enables strict parsing mode (errors on unknown codes).
	Strict bool
}

// NewParser creates a new PICS parser.
func NewParser() *Parser {
	return &Parser{
		Strict: false,
	}
}

// ParseFile parses a PICS file from the filesystem.
func (p *Parser) ParseFile(path string) (*PICS, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	return p.Parse(f)
}

// Parse parses a PICS file from a reader.
func (p *Parser) Parse(r io.Reader) (*PICS, error) {
	pics := NewPICS()
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the line
		entry, err := p.parseLine(line, lineNum)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		pics.Entries = append(pics.Entries, entry)
		pics.ByCode[entry.Code.String()] = entry

		// Track side and version
		if entry.Code.Feature == "" {
			if entry.Code.Side == SideServer {
				pics.Side = SideServer
			} else if entry.Code.Side == SideClient {
				pics.Side = SideClient
			}
		}

		// Track version
		if entry.Code.Raw == "MASH.S.VERSION" || entry.Code.Raw == "MASH.C.VERSION" {
			pics.Version = int(entry.Value.Int)
		}

		// Track features
		if entry.Code.Feature != "" && entry.Code.Type == "" && entry.Value.IsTrue() {
			pics.Features = append(pics.Features, entry.Code.Feature)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return pics, nil
}

// picsCodeRegex matches PICS codes like MASH.S.CTRL.A01.Rsp
var picsCodeRegex = regexp.MustCompile(`^MASH\.([SC])(?:\.([A-Z_]+))?(?:\.([ACFEB])([0-9A-Fa-f]+))?(?:\.(Rsp|Tx))?$`)

// parseLine parses a single PICS line.
func (p *Parser) parseLine(line string, lineNum int) (Entry, error) {
	// Split on = sign
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return Entry{}, fmt.Errorf("invalid format: expected CODE=VALUE")
	}

	codeStr := strings.TrimSpace(parts[0])
	valueStr := strings.TrimSpace(parts[1])

	// Remove inline comments from value
	if idx := strings.Index(valueStr, "#"); idx != -1 {
		valueStr = strings.TrimSpace(valueStr[:idx])
	}

	// Parse the code
	code, err := p.parseCode(codeStr)
	if err != nil {
		return Entry{}, err
	}

	// Parse the value
	value := p.parseValue(valueStr)

	return Entry{
		Code:       code,
		Value:      value,
		LineNumber: lineNum,
	}, nil
}

// parseCode parses a PICS code string.
func (p *Parser) parseCode(s string) (Code, error) {
	// Handle special cases like MASH.S.VERSION
	if s == "MASH.S" || s == "MASH.C" {
		side := SideServer
		if strings.HasSuffix(s, ".C") {
			side = SideClient
		}
		return Code{
			Raw:  s,
			Side: side,
		}, nil
	}

	if s == "MASH.S.VERSION" || s == "MASH.C.VERSION" {
		side := SideServer
		if strings.Contains(s, ".C.") {
			side = SideClient
		}
		return Code{
			Raw:     s,
			Side:    side,
			Feature: "VERSION",
		}, nil
	}

	// Handle behavior codes with string values (e.g., MASH.S.CTRL.B_LIMIT_DEFAULT)
	if strings.Contains(s, ".B_") {
		parts := strings.Split(s, ".")
		if len(parts) >= 4 && parts[0] == "MASH" {
			side := SideServer
			if parts[1] == "C" {
				side = SideClient
			}
			feature := parts[2]
			// Everything after feature is the behavior name
			behaviorName := strings.Join(parts[3:], ".")
			return Code{
				Raw:     s,
				Side:    side,
				Feature: feature,
				Type:    CodeTypeBehavior,
				ID:      behaviorName,
			}, nil
		}
	}

	// Try regex match
	matches := picsCodeRegex.FindStringSubmatch(s)
	if matches == nil {
		// Check for other patterns (e.g., MASH.S.ENDPOINTS)
		parts := strings.Split(s, ".")
		if len(parts) >= 3 && parts[0] == "MASH" {
			side := SideServer
			if parts[1] == "C" {
				side = SideClient
			}
			return Code{
				Raw:     s,
				Side:    side,
				Feature: strings.Join(parts[2:], "."),
			}, nil
		}
		return Code{}, fmt.Errorf("invalid PICS code format: %s", s)
	}

	side := SideServer
	if matches[1] == "C" {
		side = SideClient
	}

	return Code{
		Raw:       s,
		Side:      side,
		Feature:   matches[2],
		Type:      CodeType(matches[3]),
		ID:        matches[4],
		Qualifier: Qualifier(matches[5]),
	}, nil
}

// parseValue parses a PICS value string.
func (p *Parser) parseValue(s string) Value {
	// Remove quotes if present
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		s = s[1 : len(s)-1]
		return Value{
			Bool:   s != "" && s != "0" && s != "false",
			String: s,
			Raw:    s,
		}
	}

	// Try to parse as integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return Value{
			Bool:   i != 0,
			Int:    i,
			String: s,
			Raw:    s,
		}
	}

	// Try to parse as float (for things like MASH.S.CONN.JITTER_FACTOR=0.25)
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return Value{
			Bool:   f != 0,
			Int:    int64(f),
			String: s,
			Raw:    s,
		}
	}

	// Treat as string
	return Value{
		Bool:   s != "" && s != "0" && s != "false",
		String: s,
		Raw:    s,
	}
}

// ParseString parses PICS from a string.
func (p *Parser) ParseString(s string) (*PICS, error) {
	return p.Parse(strings.NewReader(s))
}

// ParseFile is a convenience function to parse a PICS file.
func ParseFile(path string) (*PICS, error) {
	return NewParser().ParseFile(path)
}

// ParseString is a convenience function to parse PICS from a string.
func ParseString(s string) (*PICS, error) {
	return NewParser().ParseString(s)
}
