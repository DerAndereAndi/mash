package pics

import (
	"sort"
	"sync"
)

// RuleRegistry manages validation rules.
type RuleRegistry struct {
	mu        sync.RWMutex
	rules     map[string]Rule
	enabled   map[string]bool
	severity  map[string]Severity
	ruleOrder []string // Maintain insertion order for deterministic iteration
}

// NewRuleRegistry creates a new rule registry.
func NewRuleRegistry() *RuleRegistry {
	return &RuleRegistry{
		rules:     make(map[string]Rule),
		enabled:   make(map[string]bool),
		severity:  make(map[string]Severity),
		ruleOrder: make([]string, 0),
	}
}

// Register adds a rule to the registry.
// The rule is enabled by default with its default severity.
func (r *RuleRegistry) Register(rule Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := rule.ID()
	if _, exists := r.rules[id]; !exists {
		r.ruleOrder = append(r.ruleOrder, id)
	}
	r.rules[id] = rule
	r.enabled[id] = true
	r.severity[id] = rule.DefaultSeverity()
}

// Enable enables a rule by ID.
func (r *RuleRegistry) Enable(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[id] = true
}

// Disable disables a rule by ID.
func (r *RuleRegistry) Disable(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[id] = false
}

// SetSeverity overrides the severity for a rule.
func (r *RuleRegistry) SetSeverity(id string, severity Severity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.severity[id] = severity
}

// IsEnabled returns true if the rule is enabled.
func (r *RuleRegistry) IsEnabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled[id]
}

// GetSeverity returns the effective severity for a rule.
func (r *RuleRegistry) GetSeverity(id string) Severity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if sev, ok := r.severity[id]; ok {
		return sev
	}
	if rule, ok := r.rules[id]; ok {
		return rule.DefaultSeverity()
	}
	return SeverityError
}

// GetRule returns a rule by ID, or nil if not found.
func (r *RuleRegistry) GetRule(id string) Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rules[id]
}

// EnabledRules returns all enabled rules in registration order.
func (r *RuleRegistry) EnabledRules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var rules []Rule
	for _, id := range r.ruleOrder {
		if r.enabled[id] {
			rules = append(rules, r.rules[id])
		}
	}
	return rules
}

// AllRules returns all registered rules in registration order.
func (r *RuleRegistry) AllRules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]Rule, len(r.ruleOrder))
	for i, id := range r.ruleOrder {
		rules[i] = r.rules[id]
	}
	return rules
}

// RulesByCategory returns all rules in a category.
func (r *RuleRegistry) RulesByCategory(category string) []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var rules []Rule
	for _, id := range r.ruleOrder {
		if r.rules[id].Category() == category {
			rules = append(rules, r.rules[id])
		}
	}
	return rules
}

// Categories returns all unique categories.
func (r *RuleRegistry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	catSet := make(map[string]struct{})
	for _, rule := range r.rules {
		catSet[rule.Category()] = struct{}{}
	}

	categories := make([]string, 0, len(catSet))
	for cat := range catSet {
		categories = append(categories, cat)
	}
	sort.Strings(categories)
	return categories
}

// RunRules executes all enabled rules against a PICS and returns violations.
// The violations are updated with the registry's severity settings.
func (r *RuleRegistry) RunRules(pics *PICS) []Violation {
	rules := r.EnabledRules()
	var violations []Violation

	for _, rule := range rules {
		ruleViolations := rule.Check(pics)
		for _, v := range ruleViolations {
			// Apply registry severity override
			v.Severity = r.GetSeverity(v.RuleID)
			violations = append(violations, v)
		}
	}

	return violations
}

// Count returns the number of registered rules.
func (r *RuleRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.rules)
}

// EnabledCount returns the number of enabled rules.
func (r *RuleRegistry) EnabledCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, enabled := range r.enabled {
		if enabled {
			count++
		}
	}
	return count
}

// EnableAll enables all registered rules.
func (r *RuleRegistry) EnableAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.rules {
		r.enabled[id] = true
	}
}

// DisableAll disables all registered rules.
func (r *RuleRegistry) DisableAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.rules {
		r.enabled[id] = false
	}
}

// EnableCategory enables all rules in a category.
func (r *RuleRegistry) EnableCategory(category string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rule := range r.rules {
		if rule.Category() == category {
			r.enabled[id] = true
		}
	}
}

// DisableCategory disables all rules in a category.
func (r *RuleRegistry) DisableCategory(category string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rule := range r.rules {
		if rule.Category() == category {
			r.enabled[id] = false
		}
	}
}
