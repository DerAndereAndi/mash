package inspect

import (
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// Formatter formats inspection output.
type Formatter struct {
	// ShowMetadata includes type, access, and unit information
	ShowMetadata bool

	// ShowIDs includes numeric IDs alongside names
	ShowIDs bool

	// IndentWidth is the number of spaces per indent level
	IndentWidth int
}

// NewFormatter creates a new Formatter with default settings.
func NewFormatter() *Formatter {
	return &Formatter{
		ShowMetadata: true,
		ShowIDs:      false,
		IndentWidth:  2,
	}
}

// Indent returns the content with indentation.
func (f *Formatter) Indent(depth int, content string) string {
	width := f.IndentWidth
	if width == 0 {
		width = 2
	}
	indent := strings.Repeat(" ", depth*width)
	return indent + content
}

// FormatValue formats a value for display, including unit conversions.
func (f *Formatter) FormatValue(value any, unit string) string {
	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"

	case string:
		return fmt.Sprintf("%q", v)

	case int64:
		return f.formatInt64WithUnit(v, unit)

	case int32:
		return f.formatInt64WithUnit(int64(v), unit)

	case int:
		return f.formatInt64WithUnit(int64(v), unit)

	case uint64:
		return f.formatUint64WithUnit(v, unit)

	case uint32:
		return f.formatUint64WithUnit(uint64(v), unit)

	case uint16:
		return f.formatUint64WithUnit(uint64(v), unit)

	case uint8:
		return f.formatUint64WithUnit(uint64(v), unit)

	case float64:
		if unit != "" {
			return fmt.Sprintf("%.2f %s", v, unit)
		}
		return fmt.Sprintf("%.2f", v)

	case []byte:
		return fmt.Sprintf("0x%x", v)

	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatInt64WithUnit formats an int64 with optional unit and human-readable conversion.
func (f *Formatter) formatInt64WithUnit(v int64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%d", v)
	}

	base := fmt.Sprintf("%d %s", v, unit)

	// Add human-readable conversion for common units
	switch unit {
	case "mW":
		return fmt.Sprintf("%s (%s)", base, FormatPowerHumanReadable(v))
	case "mA":
		return fmt.Sprintf("%s (%.1f A)", base, float64(v)/1000.0)
	case "mWh":
		return fmt.Sprintf("%s (%.1f kWh)", base, float64(v)/1000000.0)
	default:
		return base
	}
}

// formatUint64WithUnit formats a uint64 with optional unit.
func (f *Formatter) formatUint64WithUnit(v uint64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%d %s", v, unit)
}

// FormatPowerHumanReadable formats power in mW to a human-readable string.
func FormatPowerHumanReadable(mW int64) string {
	if mW == 0 {
		return "0 W"
	}

	absW := float64(mW) / 1000.0
	if absW >= 1000 || absW <= -1000 {
		// Display in kW
		return fmt.Sprintf("%.1f kW", absW/1000.0)
	}
	// Display in W
	return fmt.Sprintf("%.1f W", absW)
}

// FormatOperatingState formats an operating state value.
func FormatOperatingState(state uint8) string {
	switch state {
	case 0:
		return "UNKNOWN"
	case 1:
		return "OFFLINE"
	case 2:
		return "STANDBY"
	case 3:
		return "STARTING"
	case 4:
		return "RUNNING"
	case 5:
		return "PAUSED"
	case 6:
		return "SHUTTING_DOWN"
	case 7:
		return "FAULT"
	case 8:
		return "MAINTENANCE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", state)
	}
}

// FormatControlState formats a control state value.
func FormatControlState(state uint8) string {
	switch state {
	case 0:
		return "AUTONOMOUS"
	case 1:
		return "CONTROLLED"
	case 2:
		return "LIMITED"
	case 3:
		return "FAILSAFE"
	case 4:
		return "OVERRIDE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", state)
	}
}

// FormatDirection formats a power direction value.
func FormatDirection(dir uint8) string {
	switch dir {
	case 0:
		return "CONSUMPTION"
	case 1:
		return "PRODUCTION"
	case 2:
		return "BIDIRECTIONAL"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", dir)
	}
}

// FormatEndpointType formats an endpoint type for display.
func FormatEndpointType(epType uint8) string {
	return model.EndpointType(epType).String()
}

// FormatFeatureType formats a feature type for display.
func FormatFeatureType(featType uint8) string {
	return model.FeatureType(featType).String()
}

// FormatAccess formats an access level for display.
func FormatAccess(access model.Access) string {
	switch access {
	case model.AccessReadOnly:
		return "read-only"
	case model.AccessRead:
		return "read"
	case model.AccessWrite:
		return "write"
	case model.AccessReadWrite:
		return "read-write"
	default:
		return fmt.Sprintf("access(%d)", access)
	}
}

// FormatDataType formats a data type for display.
func FormatDataType(dt model.DataType) string {
	switch dt {
	case model.DataTypeBool:
		return "bool"
	case model.DataTypeUint8:
		return "uint8"
	case model.DataTypeUint16:
		return "uint16"
	case model.DataTypeUint32:
		return "uint32"
	case model.DataTypeUint64:
		return "uint64"
	case model.DataTypeInt8:
		return "int8"
	case model.DataTypeInt16:
		return "int16"
	case model.DataTypeInt32:
		return "int32"
	case model.DataTypeInt64:
		return "int64"
	case model.DataTypeFloat32:
		return "float32"
	case model.DataTypeFloat64:
		return "float64"
	case model.DataTypeString:
		return "string"
	case model.DataTypeBytes:
		return "bytes"
	case model.DataTypeArray:
		return "array"
	case model.DataTypeMap:
		return "map"
	default:
		return fmt.Sprintf("type(%d)", dt)
	}
}

// FormatFeatureMap formats a feature map bitmask.
func FormatFeatureMap(fm uint32) string {
	if fm == 0 {
		return "0x0 (none)"
	}
	return fmt.Sprintf("0x%08x", fm)
}

// AttributeRow represents a formatted attribute for display.
type AttributeRow struct {
	ID     uint16
	Name   string
	Value  string
	Type   string
	Access string
	Unit   string
}

// FormatAttributeTable formats a list of attributes as a table.
func (f *Formatter) FormatAttributeTable(rows []AttributeRow) string {
	if len(rows) == 0 {
		return "  (no attributes)"
	}

	var sb strings.Builder
	for _, row := range rows {
		if f.ShowIDs {
			sb.WriteString(fmt.Sprintf("  [%d] %s: %s", row.ID, row.Name, row.Value))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: %s", row.Name, row.Value))
		}
		if f.ShowMetadata && row.Type != "" {
			sb.WriteString(fmt.Sprintf(" (%s, %s)", row.Type, row.Access))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
