package model

import (
	"context"
	"errors"
)

// Command ID ranges (convention).
const (
	// CmdIDPrimary is the start of primary commands (1-4).
	CmdIDPrimary uint8 = 1

	// CmdIDSecondary is the start of secondary commands (5-8).
	CmdIDSecondary uint8 = 5

	// CmdIDControl is the start of control commands (9-12).
	CmdIDControl uint8 = 9

	// CmdIDProcess is the start of process commands (13-16).
	CmdIDProcess uint8 = 13
)

// Command errors.
var (
	ErrCommandNotFound    = errors.New("command not found")
	ErrCommandFailed      = errors.New("command execution failed")
	ErrInvalidParameters  = errors.New("invalid command parameters")
	ErrCommandNotAllowed  = errors.New("command not allowed")
)

// CommandHandler is the function signature for command handlers.
// The parameters map contains command-specific parameters.
// Returns a result map (may be nil) or an error.
type CommandHandler func(ctx context.Context, params map[string]any) (map[string]any, error)

// CommandMetadata describes a command's properties.
type CommandMetadata struct {
	// ID is the command identifier within the feature.
	ID uint8

	// Name is the human-readable command name.
	Name string

	// Description is a human-readable description.
	Description string

	// Parameters describes the expected parameters.
	Parameters []ParameterMetadata

	// Response describes the response fields.
	Response []ParameterMetadata
}

// ParameterMetadata describes a command parameter or response field.
type ParameterMetadata struct {
	// Name is the parameter name.
	Name string

	// Type is the data type.
	Type DataType

	// Required indicates if the parameter is mandatory.
	Required bool

	// Description is a human-readable description.
	Description string
}

// Command represents a command instance with its handler.
type Command struct {
	metadata *CommandMetadata
	handler  CommandHandler
}

// NewCommand creates a new command with the given metadata and handler.
func NewCommand(meta *CommandMetadata, handler CommandHandler) *Command {
	return &Command{
		metadata: meta,
		handler:  handler,
	}
}

// ID returns the command ID.
func (c *Command) ID() uint8 {
	return c.metadata.ID
}

// Metadata returns the command metadata.
func (c *Command) Metadata() *CommandMetadata {
	return c.metadata
}

// Invoke executes the command with the given parameters.
func (c *Command) Invoke(ctx context.Context, params map[string]any) (map[string]any, error) {
	// Validate required parameters
	if err := c.validateParameters(params); err != nil {
		return nil, err
	}

	// Execute handler
	if c.handler == nil {
		return nil, ErrCommandNotFound
	}

	return c.handler(ctx, params)
}

// validateParameters checks that all required parameters are present.
func (c *Command) validateParameters(params map[string]any) error {
	for _, p := range c.metadata.Parameters {
		if p.Required {
			if _, exists := params[p.Name]; !exists {
				return ErrInvalidParameters
			}
		}
	}
	return nil
}

// SetHandler sets or replaces the command handler.
func (c *Command) SetHandler(handler CommandHandler) {
	c.handler = handler
}
