package model

// UseCaseDecl declares a use case contract on the wire.
// Serialized as CBOR with integer keys for compactness.
type UseCaseDecl struct {
	EndpointID uint8  `cbor:"1,keyasint"`
	Name       string `cbor:"2,keyasint"`
	Major      uint8  `cbor:"3,keyasint"`
	Minor      uint8  `cbor:"4,keyasint"`
}
