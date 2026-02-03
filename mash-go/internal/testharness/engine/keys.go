package engine

// Infrastructure keys used internally by the engine.
const (
	InternalStepOutput = "__step_output"
)

// State keys referenced by engine checkers. These correspond to output keys
// set by runner handlers and looked up by engine-level checkers.
const (
	KeyIssuerFingerprint = "issuer_fingerprint"
	KeyFingerprint       = "fingerprint"
)

// Checker registration names -- the string values that appear in YAML test
// files and are used as map keys in Engine.checkers.
const (
	CheckerNameDefault                 = "default"
	CheckerNameValueGreaterThan        = "value_greater_than"
	CheckerNameValueLessThan           = "value_less_than"
	CheckerNameValueInRange            = "value_in_range"
	CheckerNameValueIsNull             = "value_is_null"
	CheckerNameValueIsMap              = "value_is_map"
	CheckerNameContains                = "contains"
	CheckerNameMapSizeEquals           = "map_size_equals"
	CheckerNameSaveAs                  = "save_as"
	CheckerNameValueEquals             = "value_equals"
	CheckerNameIssuerFingerprintEquals = "issuer_fingerprint_equals"
)
