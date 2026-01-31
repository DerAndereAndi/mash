package pics

import (
	"fmt"
	"sort"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// GenerateUseCaseCodes produces PICS entries from use case declarations.
// Each unique use case name produces a MASH.<side>.UC.<name>=1 entry.
// Duplicate names (e.g., same UC on multiple endpoints) are deduplicated.
// Results are sorted by code string for determinism.
func GenerateUseCaseCodes(decls []*model.UseCaseDecl, side Side) []Entry {
	seen := make(map[string]bool)
	var entries []Entry

	for _, decl := range decls {
		if seen[decl.Name] {
			continue
		}
		seen[decl.Name] = true

		codeStr := fmt.Sprintf("MASH.%s.UC.%s", side, decl.Name)
		entries = append(entries, Entry{
			Code: Code{
				Raw:     codeStr,
				Side:    side,
				Feature: fmt.Sprintf("UC.%s", decl.Name),
			},
			Value: Value{
				Bool:   true,
				Int:    1,
				String: "1",
				Raw:    "1",
			},
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Code.String() < entries[j].Code.String()
	})

	return entries
}
