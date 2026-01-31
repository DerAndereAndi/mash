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
		if entry.Code.Feature == "" && entry.Code.EndpointID == 0 {
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

		// Populate endpoints
		p.trackEndpoint(pics, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	return pics, nil
}

// picsCodeRegex matches PICS codes like MASH.S.CTRL.A01.Rsp and MASH.S.E01.CTRL.A01.Rsp
// Groups: 1=Side, 2=Endpoint(optional), 3=Feature(optional), 4=Type(optional), 5=ID(optional), 6=Qualifier(optional)
var picsCodeRegex = regexp.MustCompile(`^MASH\.([SC])(?:\.(E[0-9A-Fa-f]{2}))?(?:\.([A-Z_]+))?(?:\.([ACFEB])([0-9A-Fa-f]+))?(?:\.(Rsp|Tx))?$`)

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

	// Handle behavior codes with string values (e.g., MASH.S.CTRL.B_LIMIT_DEFAULT
	// and MASH.S.E01.CTRL.B_LIMIT_DEFAULT)
	if strings.Contains(s, ".B_") {
		parts := strings.Split(s, ".")
		if len(parts) >= 4 && parts[0] == "MASH" {
			side := SideServer
			if parts[1] == "C" {
				side = SideClient
			}
			idx := 2
			var epID uint8
			// Check for endpoint segment
			if len(parts[idx]) == 3 && parts[idx][0] == 'E' {
				id, err := strconv.ParseUint(parts[idx][1:], 16, 8)
				if err == nil {
					epID = uint8(id)
					idx++
				}
			}
			if idx < len(parts)-1 {
				feature := parts[idx]
				behaviorName := strings.Join(parts[idx+1:], ".")
				return Code{
					Raw:        s,
					Side:       side,
					EndpointID: epID,
					Feature:    feature,
					Type:       CodeTypeBehavior,
					ID:         behaviorName,
				}, nil
			}
		}
	}

	// Try regex match
	matches := picsCodeRegex.FindStringSubmatch(s)
	if matches == nil {
		// Check for other patterns (e.g., MASH.S.ENDPOINTS, MASH.S.CONN.MAX_CONNECTIONS)
		parts := strings.Split(s, ".")
		if len(parts) >= 3 && parts[0] == "MASH" {
			side := SideServer
			if parts[1] == "C" {
				side = SideClient
			}
			idx := 2
			var epID uint8
			// Check for endpoint segment
			if len(parts[idx]) == 3 && parts[idx][0] == 'E' {
				id, err := strconv.ParseUint(parts[idx][1:], 16, 8)
				if err == nil {
					epID = uint8(id)
					idx++
				}
			}
			return Code{
				Raw:        s,
				Side:       side,
				EndpointID: epID,
				Feature:    strings.Join(parts[idx:], "."),
			}, nil
		}
		return Code{}, fmt.Errorf("invalid PICS code format: %s", s)
	}

	side := SideServer
	if matches[1] == "C" {
		side = SideClient
	}

	var endpointID uint8
	if matches[2] != "" {
		// Parse endpoint hex (E01 -> 1)
		id, err := strconv.ParseUint(matches[2][1:], 16, 8)
		if err != nil {
			return Code{}, fmt.Errorf("invalid endpoint ID in %s: %w", s, err)
		}
		endpointID = uint8(id)
	}

	return Code{
		Raw:        s,
		Side:       side,
		EndpointID: endpointID,
		Feature:    matches[3],
		Type:       CodeType(matches[4]),
		ID:         matches[5],
		Qualifier:  Qualifier(matches[6]),
	}, nil
}

// trackEndpoint populates endpoint data from a parsed entry.
func (p *Parser) trackEndpoint(pics *PICS, entry Entry) {
	epID := entry.Code.EndpointID
	if epID > 0 {
		// Ensure endpoint exists in map
		if pics.Endpoints[epID] == nil {
			pics.Endpoints[epID] = &EndpointPICS{ID: epID}
		}
		ep := pics.Endpoints[epID]

		// Endpoint type declaration: MASH.S.E01=INVERTER (Feature is empty, Type is empty)
		if entry.Code.Feature == "" && entry.Code.Type == "" {
			ep.Type = entry.Value.String
		}

		// Track per-endpoint features (feature presence code with no Type)
		if entry.Code.Feature != "" && entry.Code.Type == "" && entry.Value.IsTrue() {
			// Avoid duplicates
			found := false
			for _, f := range ep.Features {
				if f == entry.Code.Feature {
					found = true
					break
				}
			}
			if !found {
				ep.Features = append(ep.Features, entry.Code.Feature)
			}
		}
	} else {
		// Device-level feature tracking (transport/pairing features)
		if entry.Code.Feature != "" && entry.Code.Type == "" && entry.Value.IsTrue() {
			pics.Features = append(pics.Features, entry.Code.Feature)
		}
	}
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
