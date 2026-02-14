package main

import (
	"context"
	"log"
	"time"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// Simulation control state.
var (
	simCtx       context.Context
	simCancel    context.CancelFunc
	simRunning   bool
	connectedCnt int
)

func startSimulation() {
	if simRunning {
		return
	}
	simCtx, simCancel = context.WithCancel(context.Background())
	simRunning = true
	go runSimulation(simCtx, config.Type)
	log.Println("[SIM] Simulation started")
}

func stopSimulation() {
	if !simRunning {
		return
	}
	if simCancel != nil {
		simCancel()
	}
	simRunning = false
	log.Println("[SIM] Simulation stopped")
}

func runSimulation(ctx context.Context, deviceType DeviceType) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var power int64

	// Attribute IDs from features package
	const (
		attrACActivePower = uint16(1)  // features.MeasurementAttrACActivePower
		attrDCPower       = uint16(40) // features.MeasurementAttrDCPower
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var attrID uint16
			switch deviceType {
			case DeviceTypeEVSE:
				// Simulate varying charging power
				power = (power + 1000000) % 22000000
				if power == 0 {
					power = 1380000
				}
				attrID = attrACActivePower
				log.Printf("[SIM] EVSE charging at %.1f kW", float64(power)/1000000)

			case DeviceTypeInverter:
				// Simulate varying PV production based on time
				hour := time.Now().Hour()
				if hour >= 6 && hour <= 20 {
					// Daytime - produce power (negative = production)
					power = -int64((10 - abs(hour-13)) * 1000000)
				} else {
					power = 0
				}
				attrID = attrACActivePower
				log.Printf("[SIM] Inverter producing %.1f kW", float64(-power)/1000000)

			case DeviceTypeBattery:
				// Simulate charge/discharge cycles
				power = (power + 500000) % 10000000 - 5000000
				attrID = attrDCPower
				if power > 0 {
					log.Printf("[SIM] Battery charging at %.1f kW", float64(power)/1000000)
				} else if power < 0 {
					log.Printf("[SIM] Battery discharging at %.1f kW", float64(-power)/1000000)
				} else {
					log.Println("[SIM] Battery idle")
				}

			case DeviceTypeHeatPump:
				// Simulate varying heating power
				power = (power + 500000) % 8000000
				if power == 0 {
					power = 1500000 // Minimum 1.5 kW
				}
				attrID = attrACActivePower
				log.Printf("[SIM] Heat pump consuming %.1f kW", float64(power)/1000000)
			}

			// Update the attribute and notify subscribed zones
			// Endpoint 1 = functional endpoint, FeatureMeasurement = 0x0002
			if deviceSvc != nil && attrID != 0 {
				if err := deviceSvc.NotifyAttributeChange(1, uint8(model.FeatureMeasurement), attrID, power); err != nil {
					log.Printf("[SIM] Failed to notify attribute change: %v", err)
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
