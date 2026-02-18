package runner

import (
	"strings"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

func decideCommissionZoneType(explicit cert.ZoneType, suiteExists bool, current cert.ZoneType) cert.ZoneType {
	if explicit != 0 {
		return explicit
	}
	if suiteExists && current == cert.ZoneTypeTest {
		return cert.ZoneTypeLocal
	}
	if current != 0 {
		return current
	}
	return cert.ZoneTypeLocal
}

func parseRequestedZoneType(params map[string]any) (string, cert.ZoneType) {
	if params == nil {
		return "", 0
	}
	raw, ok := params[KeyZoneType].(string)
	if !ok || raw == "" {
		return "", 0
	}
	zoneType := strings.ToUpper(raw)
	switch zoneType {
	case "GRID":
		return zoneType, cert.ZoneTypeGrid
	case "LOCAL":
		return zoneType, cert.ZoneTypeLocal
	case "TEST":
		return zoneType, cert.ZoneTypeTest
	default:
		return zoneType, 0
	}
}

func (r *Runner) applyCommissionZoneType(params map[string]any) string {
	requestedZoneType, explicit := parseRequestedZoneType(params)
	current := r.connMgr.CommissionZoneType()
	effective := decideCommissionZoneType(explicit, r.suite.ZoneID() != "", current)
	if effective != current {
		r.connMgr.SetCommissionZoneType(effective)
	}
	return requestedZoneType
}
