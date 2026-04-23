package pics

import (
	"fmt"
	"sort"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

// GenerateUseCaseCodes produces PICS entries from use case declarations.
// Each unique use case ID produces a MASH.<side>.UC.<name>=1 entry.
// Scenario entries are generated as MASH.<side>.UC.<name>.S<hex>=1 for each set bit.
// Duplicate IDs (e.g., same UC on multiple endpoints) are deduplicated.
// Results are sorted by code string for determinism.
func GenerateUseCaseCodes(decls []*model.UseCaseDecl, side Side) []Entry {
	seen := make(map[uint16]bool)
	var entries []Entry

	for _, decl := range decls {
		if seen[decl.ID] {
			continue
		}
		seen[decl.ID] = true

		// Look up name from ID
		name, ok := usecase.IDToName[usecase.UseCaseID(decl.ID)]
		if !ok {
			name = usecase.UseCaseName(fmt.Sprintf("0x%02X", decl.ID))
		}

		// Use case presence entry
		codeStr := fmt.Sprintf("MASH.%s.UC.%s", side, name)
		entries = append(entries, Entry{
			Code: Code{
				Raw:     codeStr,
				Side:    side,
				Feature: fmt.Sprintf("UC.%s", name),
			},
			Value: Value{
				Bool:   true,
				Int:    1,
				String: "1",
				Raw:    "1",
			},
		})

		// Scenario entries for each set bit
		for bit := uint8(0); bit < 32; bit++ {
			if decl.Scenarios&(1<<bit) != 0 {
				scenarioCode := fmt.Sprintf("MASH.%s.UC.%s.S%02X", side, name, bit)
				entries = append(entries, Entry{
					Code: Code{
						Raw:     scenarioCode,
						Side:    side,
						Feature: fmt.Sprintf("UC.%s.S%02X", name, bit),
					},
					Value: Value{
						Bool:   true,
						Int:    1,
						String: "1",
						Raw:    "1",
					},
				})
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Code.String() < entries[j].Code.String()
	})

	return entries
}
