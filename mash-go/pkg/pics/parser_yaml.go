package pics

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlPICS represents the YAML structure of a PICS file.
type yamlPICS struct {
	Device *yamlDevice        `yaml:"device"`
	Items  map[string]any     `yaml:"items"`
}

// yamlDevice represents the device metadata in YAML format.
type yamlDevice struct {
	Vendor  string `yaml:"vendor"`
	Product string `yaml:"product"`
	Model   string `yaml:"model"`
	Version string `yaml:"version"`
}

// parseYAML parses PICS data in YAML format.
func (p *Parser) parseYAML(data []byte) (*PICS, error) {
	pics := NewPICS()
	pics.Format = FormatYAML

	// First, parse to get structure and values
	var y yamlPICS
	if err := yaml.Unmarshal(data, &y); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	// Extract device metadata if present
	if y.Device != nil {
		pics.Device = &DeviceMetadata{
			Vendor:  y.Device.Vendor,
			Product: y.Device.Product,
			Model:   y.Device.Model,
			Version: y.Device.Version,
		}
	}

	// Parse items with line numbers using yaml.Node
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("YAML node parse error: %w", err)
	}

	// Find the items node and extract line numbers
	lineNumbers := make(map[string]int)
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		doc := root.Content[0]
		if doc.Kind == yaml.MappingNode {
			for i := 0; i < len(doc.Content)-1; i += 2 {
				keyNode := doc.Content[i]
				valueNode := doc.Content[i+1]
				if keyNode.Value == "items" && valueNode.Kind == yaml.MappingNode {
					for j := 0; j < len(valueNode.Content)-1; j += 2 {
						itemKey := valueNode.Content[j]
						lineNumbers[itemKey.Value] = itemKey.Line
					}
				}
			}
		}
	}

	// Process items
	for key, value := range y.Items {
		lineNum := lineNumbers[key]
		entry, err := p.parseYAMLItem(key, value, lineNum)
		if err != nil {
			if lineNum > 0 {
				return nil, fmt.Errorf("line %d: %w", lineNum, err)
			}
			return nil, err
		}

		pics.Entries = append(pics.Entries, entry)
		pics.ByCode[entry.Code.String()] = entry

		// Track side
		if entry.Code.Feature == "" && entry.Code.EndpointID == 0 {
			switch entry.Code.Side {
			case SideServer:
				pics.Side = SideServer
			case SideClient:
				pics.Side = SideClient
			}
		}

		// Populate endpoints
		p.trackEndpoint(pics, entry)
	}

	return pics, nil
}

// parseYAMLItem parses a single YAML item into an Entry.
func (p *Parser) parseYAMLItem(key string, value any, lineNum int) (Entry, error) {
	// Convert the key to a MASH code
	code, err := p.parseYAMLCode(key)
	if err != nil {
		return Entry{}, err
	}

	// Parse the value
	picsValue := p.parseYAMLValue(value)

	return Entry{
		Code:       code,
		Value:      picsValue,
		LineNumber: lineNum,
	}, nil
}

// parseYAMLCode parses a YAML key into a PICS Code.
func (p *Parser) parseYAMLCode(key string) (Code, error) {
	if strings.HasPrefix(key, "MASH.") {
		return p.parseCode(key)
	}

	// Accept device-capability (D.*) and controller-capability (C.*) codes.
	// These are shorthand flags used in PICS requirements (e.g., D.COMM.PASE,
	// C.BIDIR.EXPOSE) and are stored as-is without full MASH code parsing.
	if strings.HasPrefix(key, "D.") || strings.HasPrefix(key, "C.") {
		return Code{
			Raw: key,
		}, nil
	}

	return Code{}, fmt.Errorf("invalid PICS code format: %s (expected MASH.*, D.*, or C.*)", key)
}

// parseYAMLValue converts a YAML value to a PICS Value.
func (p *Parser) parseYAMLValue(v any) Value {
	switch val := v.(type) {
	case bool:
		raw := "0"
		if val {
			raw = "1"
		}
		return Value{
			Bool:   val,
			Int:    boolToInt(val),
			String: raw,
			Raw:    raw,
		}
	case int:
		return Value{
			Bool:   val != 0,
			Int:    int64(val),
			String: fmt.Sprintf("%d", val),
			Raw:    fmt.Sprintf("%d", val),
		}
	case int64:
		return Value{
			Bool:   val != 0,
			Int:    val,
			String: fmt.Sprintf("%d", val),
			Raw:    fmt.Sprintf("%d", val),
		}
	case float64:
		// Check if it's actually an integer
		if val == float64(int64(val)) {
			return Value{
				Bool:   val != 0,
				Int:    int64(val),
				String: fmt.Sprintf("%d", int64(val)),
				Raw:    fmt.Sprintf("%d", int64(val)),
			}
		}
		return Value{
			Bool:   val != 0,
			Int:    int64(val),
			String: fmt.Sprintf("%v", val),
			Raw:    fmt.Sprintf("%v", val),
		}
	case string:
		return p.parseValue(val)
	default:
		s := fmt.Sprintf("%v", val)
		return Value{
			Bool:   s != "" && s != "0" && s != "false",
			String: s,
			Raw:    s,
		}
	}
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
