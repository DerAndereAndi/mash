package rules

import "github.com/mash-protocol/mash-go/pkg/pics"

// RegisterAllRules registers all validation rules with the given registry.
func RegisterAllRules(registry *pics.RuleRegistry) {
	RegisterDependencyRules(registry)
	RegisterMandatoryRules(registry)
	RegisterConsistencyRules(registry)
	RegisterConformanceRules(registry)
}

// NewDefaultRegistry creates a new registry with all rules registered.
func NewDefaultRegistry() *pics.RuleRegistry {
	registry := pics.NewRuleRegistry()
	RegisterAllRules(registry)
	return registry
}
