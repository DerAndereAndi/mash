package commissioning

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// EncodePASEMessage encodes a PASE message to CBOR bytes.
func EncodePASEMessage(msg interface{}) ([]byte, error) {
	return cbor.Marshal(msg)
}

// DecodePASEMessage decodes CBOR bytes to the appropriate PASE message type.
func DecodePASEMessage(data []byte) (interface{}, error) {
	// First, decode just to get the message type
	var header struct {
		MsgType uint8 `cbor:"1,keyasint"`
	}
	if err := cbor.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}

	// Decode based on message type
	switch header.MsgType {
	case MsgPASERequest:
		var msg PASERequest
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgPASEResponse:
		var msg PASEResponse
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgPASEConfirm:
		var msg PASEConfirm
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgPASEComplete:
		var msg PASEComplete
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCSRRequest:
		var msg CSRRequest
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCSRResponse:
		var msg CSRResponse
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCertInstall:
		var msg CertInstall
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCertInstallResponse:
		var msg CertInstallResponse
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCommissioningComplete:
		var msg CommissioningComplete
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCommissioningError:
		var msg CommissioningError
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	default:
		return nil, fmt.Errorf("%w: unknown message type %d", ErrInvalidMessage, header.MsgType)
	}
}

// MessageType returns the message type from a decoded message.
func MessageType(msg interface{}) uint8 {
	switch m := msg.(type) {
	case *PASERequest:
		return m.MsgType
	case *PASEResponse:
		return m.MsgType
	case *PASEConfirm:
		return m.MsgType
	case *PASEComplete:
		return m.MsgType
	case *CSRRequest:
		return m.MsgType
	case *CSRResponse:
		return m.MsgType
	case *CertInstall:
		return m.MsgType
	case *CertInstallResponse:
		return m.MsgType
	case *CommissioningComplete:
		return m.MsgType
	case *CommissioningError:
		return m.MsgType
	default:
		return 0
	}
}
