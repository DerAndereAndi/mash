package model

// UseCaseDecl declares a use case contract on the wire.
// Serialized as CBOR with integer keys for compactness.
type UseCaseDecl struct {
	EndpointID uint8  `cbor:"1,keyasint"`
	ID         uint16 `cbor:"2,keyasint"` // use case identifier (see protocol-versions.yaml)
	Major      uint8  `cbor:"3,keyasint"`
	Minor      uint8  `cbor:"4,keyasint"`
	Scenarios  uint32 `cbor:"5,keyasint"` // bitmap of supported scenarios
}
