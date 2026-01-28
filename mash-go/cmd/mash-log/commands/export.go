package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mash-protocol/mash-go/pkg/log"
)

// RunExport exports the log file to the specified format.
func RunExport(path, format, output string) error {
	reader, err := log.NewReader(path)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer reader.Close()

	// Determine output writer
	var w io.Writer = os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	switch format {
	case "jsonl":
		return exportJSONL(reader, w)
	case "csv":
		return exportCSV(reader, w)
	default:
		return fmt.Errorf("unknown format: %s (supported: jsonl, csv)", format)
	}
}

func exportJSONL(reader *log.Reader, w io.Writer) error {
	encoder := json.NewEncoder(w)
	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read event: %w", err)
		}
		if err := encoder.Encode(event); err != nil {
			return fmt.Errorf("failed to encode event: %w", err)
		}
	}
	return nil
}

func exportCSV(reader *log.Reader, w io.Writer) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Write header
	header := []string{"timestamp", "connection_id", "direction", "layer", "category", "device_id", "zone_id", "type", "message_id"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for {
		event, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read event: %w", err)
		}

		// Determine event type
		eventType := "unknown"
		msgID := ""
		switch {
		case event.Frame != nil:
			eventType = "frame"
		case event.Message != nil:
			eventType = event.Message.Type.String()
			msgID = fmt.Sprintf("%d", event.Message.MessageID)
		case event.StateChange != nil:
			eventType = "state"
		case event.ControlMsg != nil:
			eventType = event.ControlMsg.Type.String()
		case event.Error != nil:
			eventType = "error"
		}

		row := []string{
			event.Timestamp.UTC().Format("2006-01-02T15:04:05.000000Z"),
			event.ConnectionID,
			event.Direction.String(),
			event.Layer.String(),
			event.Category.String(),
			event.DeviceID,
			event.ZoneID,
			eventType,
			msgID,
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}
	return nil
}
