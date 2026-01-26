// Package discovery implements mDNS/DNS-SD discovery for MASH devices.
//
// MASH uses three separate mDNS service types (following the Matter model):
//
// # Commissionable Discovery (_mashc._udp)
//
// Devices advertise this service when in commissioning mode (ready for pairing).
// Instance name format: MASH-<discriminator>
// TXT records include: D (discriminator), cat (device categories),
// serial, brand, model, and optionally DN (device name).
//
// # Operational Discovery (_mash._tcp)
//
// Commissioned devices advertise this service for operational communication.
// One instance per zone membership. Instance name format: <zone-id>-<device-id>
// where both IDs are fingerprint-derived from certificates (first 64 bits of SHA-256).
// TXT records include: ZI (zone ID), DI (device ID), and optionally VP, FW, FM.
//
// # Commissioner Discovery (_mashd._udp)
//
// Zone controllers advertise this service to allow device-initiated pairing.
// Instance name is the user-friendly zone name.
// TXT records include: ZN (zone name), ZI (zone ID), and optionally VP, DN, DC.
//
// # QR Code
//
// The QR code format is: MASH:<version>:<discriminator>:<setupcode>
// It contains only the minimum needed for commissioning - the discriminator
// to find the device via mDNS, and the setup code for SPAKE2+ authentication.
//
// # Device Categories
//
// Categories are aligned with EEBUS "SHIP Requirements for Installation Process":
//   - 1: Grid Connection Point Hub (GCPH)
//   - 2: Energy Management System (EMS)
//   - 3: E-mobility (EVSE, wallbox)
//   - 4: HVAC (heat pump, AC)
//   - 5: Inverter (PV, battery, hybrid)
//   - 6: Domestic appliance
//   - 7: Metering
//
// Devices may belong to multiple categories (e.g., hybrid inverter with EMS = "2,5").
package discovery
