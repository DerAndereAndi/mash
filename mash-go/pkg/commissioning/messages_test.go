package commissioning_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// TestErrorCodeZoneTypeExists_Value verifies the error code constant is 10.
func TestErrorCodeZoneTypeExists_Value(t *testing.T) {
	if commissioning.ErrCodeZoneTypeExists != 10 {
		t.Errorf("ErrCodeZoneTypeExists: expected 10, got %d", commissioning.ErrCodeZoneTypeExists)
	}
}

// TestErrorCodeString_ZoneTypeExists verifies the string representation of ZONE_TYPE_EXISTS.
func TestErrorCodeString_ZoneTypeExists(t *testing.T) {
	str := commissioning.ErrorCodeString(commissioning.ErrCodeZoneTypeExists)
	if str != "zone type already exists" {
		t.Errorf("ErrorCodeString(ErrCodeZoneTypeExists): expected 'zone type already exists', got '%s'", str)
	}
}

// TestErrorCodeString_AllCodes verifies string representation for all error codes.
func TestErrorCodeString_AllCodes(t *testing.T) {
	tests := []struct {
		code     uint8
		expected string
	}{
		{commissioning.ErrCodeSuccess, "success"},
		// DEC-047: Authentication errors now return generic "authentication failed"
		{commissioning.ErrCodeAuthFailed, "authentication failed"},
		{commissioning.ErrCodeInvalidPublicKey, "authentication failed"}, // Same as AuthFailed
		{commissioning.ErrCodeConfirmFailed, "authentication failed"},    // Generic message
		{commissioning.ErrCodeCSRFailed, "CSR generation failed"},
		{commissioning.ErrCodeCertInstallFailed, "certificate installation failed"},
		{commissioning.ErrCodeZoneTypeExists, "zone type already exists"},
		{commissioning.ErrCodeInternalError, "internal error"},
		{99, "unknown error (99)"}, // Unknown code
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := commissioning.ErrorCodeString(tt.code)
			if got != tt.expected {
				t.Errorf("ErrorCodeString(%d): expected '%s', got '%s'", tt.code, tt.expected, got)
			}
		})
	}
}

// TestCertInstallResponse_ZoneTypeExistsError verifies CertInstallResponse can encode the ZONE_TYPE_EXISTS error.
func TestCertInstallResponse_ZoneTypeExistsError(t *testing.T) {
	resp := &commissioning.CertInstallResponse{
		MsgType:   commissioning.MsgCertInstallResponse,
		ErrorCode: commissioning.ErrCodeZoneTypeExists,
	}

	// Encode
	data, err := commissioning.EncodePASEMessage(resp)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode
	decoded, err := commissioning.DecodePASEMessage(data)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	decodedResp, ok := decoded.(*commissioning.CertInstallResponse)
	if !ok {
		t.Fatalf("Expected *CertInstallResponse, got %T", decoded)
	}

	if decodedResp.ErrorCode != commissioning.ErrCodeZoneTypeExists {
		t.Errorf("ErrorCode: expected %d, got %d", commissioning.ErrCodeZoneTypeExists, decodedResp.ErrorCode)
	}
}

// TestCommissioningError_ZoneTypeExists verifies CommissioningError can use ZONE_TYPE_EXISTS.
func TestCommissioningError_ZoneTypeExists(t *testing.T) {
	errMsg := &commissioning.CommissioningError{
		MsgType:   commissioning.MsgCommissioningError,
		ErrorCode: commissioning.ErrCodeZoneTypeExists,
		Message:   "zone of this type already exists",
	}

	// Encode
	data, err := commissioning.EncodePASEMessage(errMsg)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode
	decoded, err := commissioning.DecodePASEMessage(data)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	decodedErr, ok := decoded.(*commissioning.CommissioningError)
	if !ok {
		t.Fatalf("Expected *CommissioningError, got %T", decoded)
	}

	if decodedErr.ErrorCode != commissioning.ErrCodeZoneTypeExists {
		t.Errorf("ErrorCode: expected %d, got %d", commissioning.ErrCodeZoneTypeExists, decodedErr.ErrorCode)
	}
	if decodedErr.Message != errMsg.Message {
		t.Errorf("Message: expected '%s', got '%s'", errMsg.Message, decodedErr.Message)
	}
}
