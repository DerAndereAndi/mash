package pics

import (
	"fmt"
	"strings"
)

// Severity represents the severity level of a validation issue.
type Severity int

const (
	// SeverityError indicates a critical issue that makes the PICS invalid.
	SeverityError Severity = iota
	// SeverityWarning indicates a potential issue that should be addressed.
	SeverityWarning
	// SeverityInfo indicates an informational note or suggestion.
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// Rule represents a validation rule that can be applied to a PICS.
type Rule interface {
	// ID returns the unique identifier for this rule (e.g., "DEP-001").
	ID() string
	// Name returns a human-readable name for the rule.
	Name() string
	// Category returns the rule category (e.g., "dependency", "mandatory", "consistency").
	Category() string
	// DefaultSeverity returns the default severity level.
	DefaultSeverity() Severity
	// Check applies the rule to a PICS and returns any violations.
	Check(pics *PICS) []Violation
}

// Violation represents a single rule violation found during validation.
type Violation struct {
	// RuleID is the ID of the rule that was violated.
	RuleID string
	// Severity is the severity level of this violation.
	Severity Severity
	// Message describes what went wrong.
	Message string
	// PICSCodes lists the PICS codes involved in the violation.
	PICSCodes []string
	// LineNumbers lists the source line numbers involved (if known).
	LineNumbers []int
	// Suggestion provides a suggested fix (if applicable).
	Suggestion string
}

// String returns a formatted string representation of the violation.
func (v Violation) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s: %s", v.RuleID, v.Severity, v.Message))

	if len(v.PICSCodes) > 0 {
		sb.WriteString(fmt.Sprintf(" (codes: %s)", strings.Join(v.PICSCodes, ", ")))
	}

	if len(v.LineNumbers) > 0 {
		lines := make([]string, len(v.LineNumbers))
		for i, ln := range v.LineNumbers {
			lines[i] = fmt.Sprintf("%d", ln)
		}
		sb.WriteString(fmt.Sprintf(" [lines: %s]", strings.Join(lines, ", ")))
	}

	if v.Suggestion != "" {
		sb.WriteString(fmt.Sprintf(" -> %s", v.Suggestion))
	}

	return sb.String()
}

// HasErrors returns true if any violation has severity Error.
func HasErrors(violations []Violation) bool {
	for _, v := range violations {
		if v.Severity == SeverityError {
			return true
		}
	}
	return false
}

// FilterBySeverity returns violations at or above the given severity level.
func FilterBySeverity(violations []Violation, minSeverity Severity) []Violation {
	var filtered []Violation
	for _, v := range violations {
		if v.Severity <= minSeverity {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// BaseRule provides a default implementation of common Rule methods.
type BaseRule struct {
	id              string
	name            string
	category        string
	defaultSeverity Severity
}

// ID returns the rule ID.
func (r *BaseRule) ID() string { return r.id }

// Name returns the rule name.
func (r *BaseRule) Name() string { return r.name }

// Category returns the rule category.
func (r *BaseRule) Category() string { return r.category }

// DefaultSeverity returns the default severity.
func (r *BaseRule) DefaultSeverity() Severity { return r.defaultSeverity }

// NewBaseRule creates a new BaseRule with the given properties.
func NewBaseRule(id, name, category string, severity Severity) *BaseRule {
	return &BaseRule{
		id:              id,
		name:            name,
		category:        category,
		defaultSeverity: severity,
	}
}
