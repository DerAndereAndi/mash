package pics

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// detectFormat examines the data to determine if it's key=value or YAML format.
// Returns FormatKeyValue for key=value format, FormatYAML for YAML format.
func detectFormat(data []byte) Format {
	if len(data) == 0 {
		return FormatKeyValue // Empty defaults to key=value
	}

	// Look at non-comment, non-empty lines
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)

		// Skip empty lines and comments
		if len(trimmed) == 0 || trimmed[0] == '#' {
			continue
		}

		// YAML indicators: lines starting with "device:" or "items:" or key starting with "D." or "  "
		if bytes.HasPrefix(trimmed, []byte("device:")) ||
			bytes.HasPrefix(trimmed, []byte("items:")) {
			return FormatYAML
		}

		// Key=value indicator: lines containing "=" with MASH. prefix
		if bytes.Contains(trimmed, []byte("=")) {
			if bytes.HasPrefix(trimmed, []byte("MASH.")) {
				return FormatKeyValue
			}
		}

		// YAML indicator: indented content or colon without equals
		if bytes.HasPrefix(line, []byte("  ")) {
			return FormatYAML
		}
		if bytes.Contains(trimmed, []byte(": ")) && !bytes.Contains(trimmed, []byte("=")) {
			return FormatYAML
		}
	}

	// Default to key=value if no clear indicators
	return FormatKeyValue
}

// ParseOptions configures PICS parsing behavior.
type ParseOptions struct {
	// Format specifies the input format. Use FormatAuto to auto-detect.
	Format Format
	// Strict enables strict parsing mode (errors on unknown codes).
	Strict bool
}

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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	pics, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}
	pics.SourceFile = path
	return pics, nil
}

// ParseFileWithOptions parses a PICS file with explicit options.
func (p *Parser) ParseFileWithOptions(path string, opts ParseOptions) (*PICS, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	pics, err := p.ParseBytesWithOptions(data, opts)
	if err != nil {
		return nil, err
	}
	pics.SourceFile = path
	return pics, nil
}

// Parse parses a PICS file from a reader with auto-detection.
func (p *Parser) Parse(r io.Reader) (*PICS, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}
	return p.ParseBytes(data)
}

// ParseBytes parses PICS data from a byte slice with auto-detection.
func (p *Parser) ParseBytes(data []byte) (*PICS, error) {
	return p.ParseBytesWithOptions(data, ParseOptions{Format: FormatAuto, Strict: p.Strict})
}

// ParseBytesWithOptions parses PICS data with explicit options.
func (p *Parser) ParseBytesWithOptions(data []byte, opts ParseOptions) (*PICS, error) {
	format := opts.Format
	if format == FormatAuto {
		format = detectFormat(data)
	}

	var pics *PICS
	var err error

	switch format {
	case FormatYAML:
		pics, err = p.parseYAML(data)
	default:
		pics, err = p.parseKeyValue(data)
	}

	if err != nil {
		return nil, err
	}

	pics.Format = format
	return pics, nil
}

// parseKeyValue parses PICS data in key=value format.
func (p *Parser) parseKeyValue(data []byte) (*PICS, error) {
	pics := NewPICS()
	pics.Format = FormatKeyValue
	scanner := bufio.NewScanner(bytes.NewReader(data))
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
			switch entry.Code.Side {
			case SideServer:
				pics.Side = SideServer
			case SideClient:
				pics.Side = SideClient
			}
		}

		// Track version
		if entry.Code.Raw == "MASH.S.VERSION" || entry.Code.Raw == "MASH.C.VERSION" {
			pics.Version = entry.Value.Raw
		}

		// Track features
		if entry.Code.Feature != "" && entry.Code.Type == "" && entry.Value.IsTrue() {
			pics.Features = append(pics.Features, entry.Code.Feature)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
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

// ParseString parses PICS from a string with auto-detection.
func (p *Parser) ParseString(s string) (*PICS, error) {
	return p.ParseBytes([]byte(s))
}

// ParseStringWithOptions parses PICS from a string with explicit options.
func (p *Parser) ParseStringWithOptions(s string, opts ParseOptions) (*PICS, error) {
	return p.ParseBytesWithOptions([]byte(s), opts)
}

// ParseFile is a convenience function to parse a PICS file.
func ParseFile(path string) (*PICS, error) {
	return NewParser().ParseFile(path)
}

// ParseString is a convenience function to parse PICS from a string.
func ParseString(s string) (*PICS, error) {
	return NewParser().ParseString(s)
}

// ParseBytes is a convenience function to parse PICS from bytes.
func ParseBytes(data []byte) (*PICS, error) {
	return NewParser().ParseBytes(data)
}
