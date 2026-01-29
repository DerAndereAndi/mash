package model

import (
	"context"
	"testing"
)

func TestFeature_ReadAttributeWithContext_NoHook(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)
	attr := NewAttribute(&AttributeMetadata{
		ID:     AttrIDPrimary,
		Name:   "power",
		Type:   DataTypeInt64,
		Access: AccessReadOnly,
	})
	feature.AddAttribute(attr)
	_ = attr.SetValueInternal(int64(5000))

	val, err := feature.ReadAttributeWithContext(context.Background(), AttrIDPrimary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != int64(5000) {
		t.Errorf("expected 5000, got %v", val)
	}
}

func TestFeature_ReadAttributeWithContext_HookOverrides(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)
	attr := NewAttribute(&AttributeMetadata{
		ID:       AttrIDPrimary,
		Name:     "myLimit",
		Type:     DataTypeInt64,
		Access:   AccessReadOnly,
		Nullable: true,
	})
	feature.AddAttribute(attr)
	_ = attr.SetValueInternal(int64(9999)) // stored value

	feature.SetReadHook(func(ctx context.Context, attrID uint16) (any, bool) {
		if attrID == AttrIDPrimary {
			return int64(42), true
		}
		return nil, false
	})

	val, err := feature.ReadAttributeWithContext(context.Background(), AttrIDPrimary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != int64(42) {
		t.Errorf("expected hook value 42, got %v", val)
	}
}

func TestFeature_ReadAttributeWithContext_HookFallsThrough(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)
	attr := NewAttribute(&AttributeMetadata{
		ID:     AttrIDPrimary,
		Name:   "power",
		Type:   DataTypeInt64,
		Access: AccessReadOnly,
	})
	feature.AddAttribute(attr)
	_ = attr.SetValueInternal(int64(7777))

	feature.SetReadHook(func(ctx context.Context, attrID uint16) (any, bool) {
		// Only override a different attribute
		if attrID == AttrIDSecondary {
			return int64(1111), true
		}
		return nil, false
	})

	val, err := feature.ReadAttributeWithContext(context.Background(), AttrIDPrimary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != int64(7777) {
		t.Errorf("expected stored value 7777, got %v", val)
	}
}

func TestFeature_ReadAttributeWithContext_HookReturnsNil(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)
	attr := NewAttribute(&AttributeMetadata{
		ID:       AttrIDPrimary,
		Name:     "myLimit",
		Type:     DataTypeInt64,
		Access:   AccessReadOnly,
		Nullable: true,
	})
	feature.AddAttribute(attr)
	_ = attr.SetValueInternal(int64(9999))

	// Hook overrides with nil (zone has no limit set)
	feature.SetReadHook(func(ctx context.Context, attrID uint16) (any, bool) {
		if attrID == AttrIDPrimary {
			return nil, true
		}
		return nil, false
	})

	val, err := feature.ReadAttributeWithContext(context.Background(), AttrIDPrimary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil from hook, got %v", val)
	}
}

func TestFeature_ReadAllAttributesWithContext_HookOverrides(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)

	attr1 := NewAttribute(&AttributeMetadata{
		ID:       AttrIDPrimary,
		Name:     "effective",
		Type:     DataTypeInt64,
		Access:   AccessReadOnly,
		Nullable: true,
	})
	attr2 := NewAttribute(&AttributeMetadata{
		ID:       AttrIDPrimary + 1,
		Name:     "myValue",
		Type:     DataTypeInt64,
		Access:   AccessReadOnly,
		Nullable: true,
	})
	feature.AddAttribute(attr1)
	feature.AddAttribute(attr2)
	_ = attr1.SetValueInternal(int64(5000))
	_ = attr2.SetValueInternal(int64(3000)) // stored value, should be overridden

	feature.SetReadHook(func(ctx context.Context, attrID uint16) (any, bool) {
		if attrID == AttrIDPrimary+1 {
			return int64(8888), true
		}
		return nil, false
	})

	result := feature.ReadAllAttributesWithContext(context.Background())

	// attr1 should have stored value (hook falls through)
	if result[AttrIDPrimary] != int64(5000) {
		t.Errorf("expected attr1=5000, got %v", result[AttrIDPrimary])
	}

	// attr2 should have hook value
	if result[AttrIDPrimary+1] != int64(8888) {
		t.Errorf("expected attr2=8888 from hook, got %v", result[AttrIDPrimary+1])
	}
}

func TestFeature_ReadAttributeWithContext_NilContext(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)
	attr := NewAttribute(&AttributeMetadata{
		ID:     AttrIDPrimary,
		Name:   "power",
		Type:   DataTypeInt64,
		Access: AccessReadOnly,
	})
	feature.AddAttribute(attr)
	_ = attr.SetValueInternal(int64(1234))

	hookCalled := false
	feature.SetReadHook(func(ctx context.Context, attrID uint16) (any, bool) {
		hookCalled = true
		// Hook receives nil context, should not panic
		return nil, false
	})

	//nolint:staticcheck // intentionally passing nil context for test
	val, err := feature.ReadAttributeWithContext(nil, AttrIDPrimary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hookCalled {
		t.Error("expected hook to be called even with nil context")
	}
	if val != int64(1234) {
		t.Errorf("expected stored value 1234, got %v", val)
	}
}

func TestFeature_ReadAttributeWithContext_DynamicAttributes(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)

	// Add a custom attribute so attributeList has something beyond globals
	attr := NewAttribute(&AttributeMetadata{
		ID:     AttrIDPrimary,
		Name:   "power",
		Type:   DataTypeInt64,
		Access: AccessReadOnly,
	})
	feature.AddAttribute(attr)

	// attributeList and commandList are dynamic -- should still work with context
	val, err := feature.ReadAttributeWithContext(context.Background(), AttrIDAttributeList)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids, ok := val.([]uint16)
	if !ok {
		t.Fatalf("expected []uint16, got %T", val)
	}
	if len(ids) == 0 {
		t.Error("expected non-empty attribute list")
	}
}

func TestFeature_ReadAttributeWithContext_NotFound(t *testing.T) {
	feature := NewFeature(FeatureMeasurement, 1)

	_, err := feature.ReadAttributeWithContext(context.Background(), 9999)
	if err != ErrAttributeNotFound {
		t.Errorf("expected ErrAttributeNotFound, got %v", err)
	}
}
