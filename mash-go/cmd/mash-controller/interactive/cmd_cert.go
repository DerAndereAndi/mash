package interactive

import (
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"strings"
	"time"
)

// cmdCert handles the cert command.
// Usage:
//   - cert             - Show Zone CA and controller operational cert
//   - cert <device-id> - Show device certificate details
//   - cert --all       - Show summary table of all certificates
func (c *Controller) cmdCert(args []string) {
	// Check for --all flag
	if len(args) > 0 && args[0] == "--all" {
		c.showAllCerts()
		return
	}

	// If device ID provided, show device cert
	if len(args) > 0 {
		c.showDeviceCert(args[0])
		return
	}

	// Default: show Zone CA and controller cert
	c.showControllerCerts()
}

// showControllerCerts displays Zone CA and controller operational certificate.
func (c *Controller) showControllerCerts() {
	certStore := c.svc.GetCertStore()
	if certStore == nil {
		fmt.Fprintln(c.rl.Stdout(), "No certificate store configured")
		return
	}

	// Show Zone CA
	fmt.Fprintln(c.rl.Stdout(), "\nZone CA:")
	fmt.Fprintln(c.rl.Stdout(), "-------------------------------------------")

	zoneCA, err := certStore.GetZoneCA()
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(), "  Error: %v\n", err)
	} else {
		c.printCertInfo(zoneCA.Certificate, "  ")
		fmt.Fprintf(c.rl.Stdout(), "  Zone ID:       %s\n", zoneCA.ZoneID)
		fmt.Fprintf(c.rl.Stdout(), "  Zone Type:     %s\n", zoneCA.ZoneType.String())
	}

	// Show Controller Operational Cert
	fmt.Fprintln(c.rl.Stdout(), "\nController Operational Certificate:")
	fmt.Fprintln(c.rl.Stdout(), "-------------------------------------------")

	controllerCert, err := certStore.GetControllerCert()
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(), "  Error: %v\n", err)
	} else {
		c.printCertInfo(controllerCert.Certificate, "  ")
	}

	fmt.Fprintln(c.rl.Stdout())
}

// showDeviceCert displays a device's operational certificate.
func (c *Controller) showDeviceCert(partialID string) {
	// Resolve device ID
	deviceID := c.resolveDeviceID(partialID)
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(), "Device not found: %s\n", partialID)
		return
	}

	// Check if device exists
	device := c.svc.GetDevice(deviceID)
	if device == nil {
		fmt.Fprintf(c.rl.Stdout(), "Device not found: %s\n", partialID)
		return
	}

	// Use stored operational certificate (preferred)
	var deviceCert *x509.Certificate
	if device.OperationalCert != nil {
		deviceCert = device.OperationalCert
	} else if device.Connected {
		// Fallback to TLS peer cert if operational cert not stored
		session := c.svc.GetSession(deviceID)
		if session != nil {
			if tlsState := session.TLSConnectionState(); tlsState != nil && len(tlsState.PeerCertificates) > 0 {
				deviceCert = tlsState.PeerCertificates[0]
			}
		}
	}

	if deviceCert == nil {
		fmt.Fprintf(c.rl.Stdout(), "No certificate available for device %s\n", partialID)
		return
	}

	fmt.Fprintf(c.rl.Stdout(), "\nDevice Certificate: %s\n", shortID(deviceID))
	fmt.Fprintln(c.rl.Stdout(), "-------------------------------------------")
	c.printCertInfo(deviceCert, "  ")

	// Show SHA256 fingerprint
	fingerprint := sha256.Sum256(deviceCert.Raw)
	fmt.Fprintf(c.rl.Stdout(), "  Fingerprint:   %s\n", formatFingerprint(fingerprint[:]))

	fmt.Fprintln(c.rl.Stdout())
}

// showAllCerts displays a summary table of all certificates.
func (c *Controller) showAllCerts() {
	certStore := c.svc.GetCertStore()
	if certStore == nil {
		fmt.Fprintln(c.rl.Stdout(), "No certificate store configured")
		return
	}

	fmt.Fprintln(c.rl.Stdout(), "\nCertificate Summary:")
	fmt.Fprintln(c.rl.Stdout(), "--------------------------------------------------------------------------------")
	fmt.Fprintf(c.rl.Stdout(), "%-12s %-24s %-20s %-10s %s\n",
		"Type", "Subject", "Issuer", "Expiry", "Status")
	fmt.Fprintln(c.rl.Stdout(), "--------------------------------------------------------------------------------")

	// Zone CA
	zoneCA, err := certStore.GetZoneCA()
	if err == nil {
		c.printCertRow("Zone CA", zoneCA.Certificate)
	}

	// Controller Cert
	controllerCert, err := certStore.GetControllerCert()
	if err == nil {
		c.printCertRow("Controller", controllerCert.Certificate)
	}

	// Device Certificates - use stored operational cert (not TLS peer cert)
	devices := c.svc.GetAllDevices()
	for _, device := range devices {
		if device.OperationalCert != nil {
			// Use the stored operational certificate (received during commissioning)
			c.printCertRow("Device", device.OperationalCert)
		} else if !device.Connected {
			// Show disconnected devices with no cert info
			fmt.Fprintf(c.rl.Stdout(), "%-12s %-24s %-20s %-10s %s\n",
				"Device", shortID(device.ID), "-", "-", "disconnected")
		} else {
			// Fallback to TLS peer cert if operational cert not available
			session := c.svc.GetSession(device.ID)
			if session == nil {
				continue
			}

			tlsState := session.TLSConnectionState()
			if tlsState == nil || len(tlsState.PeerCertificates) == 0 {
				fmt.Fprintf(c.rl.Stdout(), "%-12s %-24s %-20s %-10s %s\n",
					"Device", shortID(device.ID), "-", "-", "no cert")
				continue
			}

			c.printCertRow("Device", tlsState.PeerCertificates[0])
		}
	}

	fmt.Fprintln(c.rl.Stdout(), "--------------------------------------------------------------------------------")
	fmt.Fprintln(c.rl.Stdout())
}

// printCertInfo prints detailed certificate information.
func (c *Controller) printCertInfo(cert *x509.Certificate, prefix string) {
	if cert == nil {
		fmt.Fprintf(c.rl.Stdout(), "%sNo certificate\n", prefix)
		return
	}

	// Subject
	fmt.Fprintf(c.rl.Stdout(), "%sSubject:     %s\n", prefix, cert.Subject.CommonName)

	// Organization if present
	if len(cert.Subject.Organization) > 0 {
		fmt.Fprintf(c.rl.Stdout(), "%sOrganization: %s\n", prefix, strings.Join(cert.Subject.Organization, ", "))
	}

	// Issuer
	fmt.Fprintf(c.rl.Stdout(), "%sIssuer:      %s\n", prefix, cert.Issuer.CommonName)

	// Validity period
	fmt.Fprintf(c.rl.Stdout(), "%sValid From:  %s\n", prefix, cert.NotBefore.Format("2006-01-02"))
	fmt.Fprintf(c.rl.Stdout(), "%sValid Until: %s\n", prefix, cert.NotAfter.Format("2006-01-02"))

	// Days until expiry
	daysUntil := int(time.Until(cert.NotAfter).Hours() / 24)
	status := certStatus(daysUntil)
	fmt.Fprintf(c.rl.Stdout(), "%sExpires In:  %d days [%s]\n", prefix, daysUntil, status)

	// Serial number (truncated)
	serialStr := cert.SerialNumber.String()
	if len(serialStr) > 16 {
		serialStr = serialStr[:16] + "..."
	}
	fmt.Fprintf(c.rl.Stdout(), "%sSerial:      %s\n", prefix, serialStr)

	// Is CA
	if cert.IsCA {
		fmt.Fprintf(c.rl.Stdout(), "%sIs CA:       true\n", prefix)
	} else {
		fmt.Fprintf(c.rl.Stdout(), "%sIs CA:       false\n", prefix)
	}

	// Key type
	keyType := "Unknown"
	if cert.PublicKeyAlgorithm == x509.ECDSA {
		keyType = "ECDSA P-256"
	} else if cert.PublicKeyAlgorithm == x509.RSA {
		keyType = "RSA"
	}
	fmt.Fprintf(c.rl.Stdout(), "%sKey Type:    %s\n", prefix, keyType)
}

// printCertRow prints a single row in the certificate summary table.
func (c *Controller) printCertRow(certType string, cert *x509.Certificate) {
	subject := cert.Subject.CommonName
	if len(subject) > 24 {
		subject = subject[:21] + "..."
	}

	issuer := cert.Issuer.CommonName
	if len(issuer) > 20 {
		issuer = issuer[:17] + "..."
	}

	daysUntil := int(time.Until(cert.NotAfter).Hours() / 24)
	expiry := fmt.Sprintf("%d days", daysUntil)
	status := certStatus(daysUntil)

	fmt.Fprintf(c.rl.Stdout(), "%-12s %-24s %-20s %-10s %s\n",
		certType, subject, issuer, expiry, status)
}

// certStatus returns a status string based on days until expiry.
func certStatus(daysUntil int) string {
	if daysUntil <= 0 {
		return "EXPIRED"
	}
	if daysUntil <= 7 {
		return "CRITICAL"
	}
	if daysUntil <= 30 {
		return "RENEW"
	}
	return "OK"
}

// shortID returns a shortened device ID for display.
func shortID(id string) string {
	if len(id) > 16 {
		return id[:16] + "..."
	}
	return id
}

// formatFingerprint formats a certificate fingerprint as hex with colons.
func formatFingerprint(fp []byte) string {
	var parts []string
	for i, b := range fp {
		if i > 0 && i%2 == 0 {
			parts = append(parts, ":")
		}
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	// Only show first 32 chars for brevity
	result := strings.Join(parts, "")
	if len(result) > 32 {
		result = result[:32] + "..."
	}
	return result
}
