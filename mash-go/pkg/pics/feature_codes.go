package pics

import "github.com/mash-protocol/mash-go/pkg/model"

// FeatureTypeToPICSCode maps a FeatureType numeric ID to its PICS short code.
var FeatureTypeToPICSCode = map[uint8]string{
	uint8(model.FeatureDeviceInfo):      "INFO",
	uint8(model.FeatureStatus):          "STAT",
	uint8(model.FeatureElectrical):      "ELEC",
	uint8(model.FeatureMeasurement):     "MEAS",
	uint8(model.FeatureEnergyControl):   "CTRL",
	uint8(model.FeatureChargingSession): "CHRG",
	uint8(model.FeatureTariff):          "TAR",
	uint8(model.FeatureSignals):         "SIG",
	uint8(model.FeaturePlan):            "PLAN",
	uint8(model.FeatureTestControl):     "TCTRL",
}

// PICSCodeToFeatureType maps PICS short codes back to FeatureType numeric IDs.
var PICSCodeToFeatureType = func() map[string]uint8 {
	m := make(map[string]uint8, len(FeatureTypeToPICSCode))
	for id, code := range FeatureTypeToPICSCode {
		m[code] = id
	}
	return m
}()
