package specparse

import "strings"

// FeatureDirName converts "DeviceInfo" to "device-info", "EnergyControl" to "energy-control".
func FeatureDirName(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('-')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
