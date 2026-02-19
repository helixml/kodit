package enrichment

import "testing"

func TestNewFilter(t *testing.T) {
	f := NewFilter()

	if len(f.Types()) != 0 {
		t.Errorf("Types() length = %d, want 0", len(f.Types()))
	}
	if len(f.Subtypes()) != 0 {
		t.Errorf("Subtypes() length = %d, want 0", len(f.Subtypes()))
	}
	if f.Limit() != 0 {
		t.Errorf("Limit() = %d, want 0", f.Limit())
	}
	if f.Offset() != 0 {
		t.Errorf("Offset() = %d, want 0", f.Offset())
	}
}

func TestFilter_WithType(t *testing.T) {
	f := NewFilter().WithType(TypeArchitecture)

	if len(f.Types()) != 1 {
		t.Fatalf("Types() length = %d, want 1", len(f.Types()))
	}
	if f.Types()[0] != TypeArchitecture {
		t.Errorf("Types()[0] = %v, want %v", f.Types()[0], TypeArchitecture)
	}
}

func TestFilter_WithType_Chaining(t *testing.T) {
	f := NewFilter().
		WithType(TypeArchitecture).
		WithType(TypeDevelopment)

	if len(f.Types()) != 2 {
		t.Fatalf("Types() length = %d, want 2", len(f.Types()))
	}
	if f.Types()[0] != TypeArchitecture {
		t.Errorf("Types()[0] = %v, want %v", f.Types()[0], TypeArchitecture)
	}
	if f.Types()[1] != TypeDevelopment {
		t.Errorf("Types()[1] = %v, want %v", f.Types()[1], TypeDevelopment)
	}
}

func TestFilter_WithSubtype(t *testing.T) {
	f := NewFilter().WithSubtype(SubtypePhysical)

	if len(f.Subtypes()) != 1 {
		t.Fatalf("Subtypes() length = %d, want 1", len(f.Subtypes()))
	}
	if f.Subtypes()[0] != SubtypePhysical {
		t.Errorf("Subtypes()[0] = %v, want %v", f.Subtypes()[0], SubtypePhysical)
	}
}

func TestFilter_WithSubtype_Chaining(t *testing.T) {
	f := NewFilter().
		WithSubtype(SubtypePhysical).
		WithSubtype(SubtypeDatabaseSchema)

	if len(f.Subtypes()) != 2 {
		t.Fatalf("Subtypes() length = %d, want 2", len(f.Subtypes()))
	}
}

func TestFilter_WithLimit(t *testing.T) {
	f := NewFilter().WithLimit(50)
	if f.Limit() != 50 {
		t.Errorf("Limit() = %d, want 50", f.Limit())
	}
}

func TestFilter_WithOffset(t *testing.T) {
	f := NewFilter().WithOffset(10)
	if f.Offset() != 10 {
		t.Errorf("Offset() = %d, want 10", f.Offset())
	}
}

func TestFilter_Immutability(t *testing.T) {
	base := NewFilter().WithType(TypeArchitecture)
	derived := base.WithType(TypeDevelopment)

	if len(base.Types()) != 1 {
		t.Errorf("base Types() length = %d, want 1 (immutability violated)", len(base.Types()))
	}
	if len(derived.Types()) != 2 {
		t.Errorf("derived Types() length = %d, want 2", len(derived.Types()))
	}
}

func TestFilter_Immutability_Subtypes(t *testing.T) {
	base := NewFilter().WithSubtype(SubtypePhysical)
	derived := base.WithSubtype(SubtypeDatabaseSchema)

	if len(base.Subtypes()) != 1 {
		t.Errorf("base Subtypes() length = %d, want 1 (immutability violated)", len(base.Subtypes()))
	}
	if len(derived.Subtypes()) != 2 {
		t.Errorf("derived Subtypes() length = %d, want 2", len(derived.Subtypes()))
	}
}

func TestFilter_Immutability_Limit(t *testing.T) {
	base := NewFilter().WithLimit(10)
	derived := base.WithLimit(20)

	if base.Limit() != 10 {
		t.Errorf("base Limit() = %d, want 10", base.Limit())
	}
	if derived.Limit() != 20 {
		t.Errorf("derived Limit() = %d, want 20", derived.Limit())
	}
}

func TestFilter_Immutability_Offset(t *testing.T) {
	base := NewFilter().WithOffset(5)
	derived := base.WithOffset(15)

	if base.Offset() != 5 {
		t.Errorf("base Offset() = %d, want 5", base.Offset())
	}
	if derived.Offset() != 15 {
		t.Errorf("derived Offset() = %d, want 15", derived.Offset())
	}
}

func TestFilter_CombinedBuilder(t *testing.T) {
	f := NewFilter().
		WithType(TypeDevelopment).
		WithSubtype(SubtypeSnippet).
		WithLimit(25).
		WithOffset(50)

	if len(f.Types()) != 1 || f.Types()[0] != TypeDevelopment {
		t.Errorf("unexpected Types: %v", f.Types())
	}
	if len(f.Subtypes()) != 1 || f.Subtypes()[0] != SubtypeSnippet {
		t.Errorf("unexpected Subtypes: %v", f.Subtypes())
	}
	if f.Limit() != 25 {
		t.Errorf("Limit() = %d, want 25", f.Limit())
	}
	if f.Offset() != 50 {
		t.Errorf("Offset() = %d, want 50", f.Offset())
	}
}

func TestFilter_FirstType(t *testing.T) {
	empty := NewFilter()
	if empty.FirstType() != nil {
		t.Error("FirstType() should be nil for empty filter")
	}

	f := NewFilter().WithType(TypeHistory).WithType(TypeUsage)
	ft := f.FirstType()
	if ft == nil {
		t.Fatal("FirstType() should not be nil")
	}
	if *ft != TypeHistory {
		t.Errorf("FirstType() = %v, want %v", *ft, TypeHistory)
	}
}

func TestFilter_FirstSubtype(t *testing.T) {
	empty := NewFilter()
	if empty.FirstSubtype() != nil {
		t.Error("FirstSubtype() should be nil for empty filter")
	}

	f := NewFilter().WithSubtype(SubtypeCookbook).WithSubtype(SubtypeAPIDocs)
	fs := f.FirstSubtype()
	if fs == nil {
		t.Fatal("FirstSubtype() should not be nil")
	}
	if *fs != SubtypeCookbook {
		t.Errorf("FirstSubtype() = %v, want %v", *fs, SubtypeCookbook)
	}
}

func TestFilter_PreservesOtherFields(t *testing.T) {
	f := NewFilter().
		WithType(TypeArchitecture).
		WithSubtype(SubtypePhysical).
		WithLimit(10).
		WithOffset(5)

	// Adding another type should preserve subtype, limit, and offset
	f2 := f.WithType(TypeDevelopment)
	if f2.Limit() != 10 {
		t.Errorf("Limit() = %d, want 10 after WithType", f2.Limit())
	}
	if f2.Offset() != 5 {
		t.Errorf("Offset() = %d, want 5 after WithType", f2.Offset())
	}
	if len(f2.Subtypes()) != 1 {
		t.Errorf("Subtypes() length = %d, want 1 after WithType", len(f2.Subtypes()))
	}

	// Adding another subtype should preserve types, limit, and offset
	f3 := f.WithSubtype(SubtypeDatabaseSchema)
	if len(f3.Types()) != 1 {
		t.Errorf("Types() length = %d, want 1 after WithSubtype", len(f3.Types()))
	}
	if f3.Limit() != 10 {
		t.Errorf("Limit() = %d, want 10 after WithSubtype", f3.Limit())
	}
}
