package mapper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testMapper = Mapper{
	PanicOnIncompatibleTypes: false,
	PanicOnMissingField:      false,
}

func TestPanicWhenDestIsNotPointer(t *testing.T) {
	defer func() { recover() }()
	source, dest := SourceTypeA{}, DestTypeA{}
	m := &Mapper{
		PanicOnMissingField: true,
	}
	m.Map(source, dest)

	t.Error("Should have panicked")
}

func TestDestinationIsUpdatedFromSource(t *testing.T) {
	source, dest := SourceTypeA{Foo: 42}, DestTypeA{}
	r := testMapper.Map(source, &dest)
	assert.Equal(t, 42, dest.Foo)
	assert.Empty(t, r.Errors)
}

func TestDestinationIsUpdatedFromSourceWhenSourcePassedAsPtr(t *testing.T) {
	source, dest := SourceTypeA{42, "Bar"}, DestTypeA{}
	r := testMapper.Map(&source, &dest)
	assert.Equal(t, 42, dest.Foo)
	assert.Equal(t, "Bar", dest.Bar)
	assert.Empty(t, r.Errors)
}

func TestWithNestedTypes(t *testing.T) {
	source := struct {
		Baz   string
		Child SourceTypeA
	}{}
	dest := struct {
		Baz   string
		Child DestTypeA
	}{}

	source.Baz = "Baz"
	source.Child.Bar = "Bar"
	r := testMapper.Map(&source, &dest)
	assert.Equal(t, "Baz", dest.Baz)
	assert.Equal(t, "Bar", dest.Child.Bar)
	assert.Empty(t, r.Errors)
}

func TestWithSliceTypes(t *testing.T) {
	source := struct {
		Children []SourceTypeA
	}{}
	dest := struct {
		Children []DestTypeA
	}{}
	source.Children = []SourceTypeA{
		SourceTypeA{Foo: 1},
		SourceTypeA{Foo: 2}}

	r := testMapper.Map(&source, &dest)
	assert.Equal(t, 1, dest.Children[0].Foo)
	assert.Equal(t, 2, dest.Children[1].Foo)
	assert.Empty(t, r.Errors)
}

func TestWithMultiLevelSlices(t *testing.T) {
	source := struct {
		Parents []SourceParent
	}{}
	dest := struct {
		Parents []DestParent
	}{}
	source.Parents = []SourceParent{
		SourceParent{
			Children: []SourceTypeA{
				SourceTypeA{Foo: 42},
				SourceTypeA{Foo: 43},
			},
		},
		SourceParent{
			Children: []SourceTypeA{},
		},
	}

	r := testMapper.Map(&source, &dest)
	assert.Empty(t, r.Errors)
}

func TestWithEmptySliceAndIncompatibleTypes(t *testing.T) {
	defer func() { recover() }()

	source := struct {
		Children []struct{ Foo string }
	}{}
	dest := struct {
		Children []struct{ Bar int }
	}{}

	m := &Mapper{
		PanicOnMissingField: true,
	}
	m.Map(&source, &dest)
	t.Error("Should have panicked")
}

func TestErrorWithEmptySliceAndIncompatibleTypes(t *testing.T) {
	source := struct {
		Children []struct{ Foo string }
	}{}
	dest := struct {
		Children []struct{ Bar int }
	}{}

	r := testMapper.Map(&source, &dest)

	assert.NotEmpty(t, r.Errors)
}

func TestWhenSourceIsMissingField(t *testing.T) {
	defer func() { recover() }()
	source := struct {
		A string
	}{}
	dest := struct {
		A, B string
	}{}
	m := &Mapper{
		PanicOnMissingField: true,
	}
	m.Map(&source, &dest)
	t.Error("Should have panicked")
}

func TestErrorWhenSourceIsMissingField(t *testing.T) {
	source := struct {
		A string
	}{}
	dest := struct {
		A, B string
	}{}
	r := testMapper.Map(&source, &dest)

	assert.NotEmpty(t, r.Errors)
}

func TestWithUnnamedFields(t *testing.T) {
	source := struct {
		Baz string
		SourceTypeA
	}{}
	dest := struct {
		Baz string
		DestTypeA
	}{}
	source.Baz = "Baz"
	source.SourceTypeA.Foo = 42

	m := Mapper{
		FieldNameMaps: map[string]string{
			"SourceTypeA": "DestTypeA",
		},
	}
	m.Map(&source, &dest)
	assert.Equal(t, "Baz", dest.Baz)
	assert.Equal(t, 42, dest.DestTypeA.Foo)
}

func TestWithPointerFieldsNotNil(t *testing.T) {
	source := struct {
		Foo *SourceTypeA
	}{}
	dest := struct {
		Foo *DestTypeA
	}{}
	source.Foo = nil

	r := testMapper.Map(&source, &dest)
	assert.Nil(t, dest.Foo)
	assert.Empty(t, r.Errors)
}

func TestWithPointerFieldsNil(t *testing.T) {
	source := struct {
		Foo *SourceTypeA
	}{}
	dest := struct {
		Foo *DestTypeA
	}{}
	source.Foo = &SourceTypeA{Foo: 42}

	r := testMapper.Map(&source, &dest)
	assert.NotNil(t, dest.Foo)
	assert.Equal(t, 42, dest.Foo.Foo)
	assert.Empty(t, r.Errors)
}

func TestMapFromPointerToNonPointerTypeWithData(t *testing.T) {
	source := struct {
		Foo *SourceTypeA
	}{}
	dest := struct {
		Foo DestTypeA
	}{}
	source.Foo = &SourceTypeA{Foo: 42}

	r := testMapper.Map(&source, &dest)
	assert.NotNil(t, dest.Foo)
	assert.Equal(t, 42, dest.Foo.Foo)
	assert.Empty(t, r.Errors)
}

func TestMapFromPointerToNonPointerTypeWithoutData(t *testing.T) {
	source := struct {
		Foo *SourceTypeA
	}{}
	dest := struct {
		Foo DestTypeA
	}{}
	source.Foo = nil

	r := testMapper.Map(&source, &dest)
	assert.NotNil(t, dest.Foo)
	assert.Equal(t, 0, dest.Foo.Foo)
	assert.Empty(t, r.Errors)

}

func TestMapFromPointerToNonPointerTypeWithoutDataAndIncompatibleType(t *testing.T) {
	defer func() { recover() }()
	// Just make sure we stil panic
	source := struct {
		Foo *SourceTypeA
	}{}
	dest := struct {
		Foo struct {
			Baz string
		}
	}{}
	source.Foo = nil

	m := &Mapper{
		PanicOnMissingField: true,
	}
	m.Map(&source, &dest)
	t.Error("Should have panicked")
}

func TestWhenUsingIncompatibleTypes(t *testing.T) {
	defer func() { recover() }()
	source := struct{ Foo string }{}
	dest := struct{ Foo int }{}
	m := &Mapper{
		PanicOnIncompatibleTypes: true,
	}
	m.Map(&source, &dest)
	t.Error("Should have panicked")
}

func TestErrorMapFromPointerToNonPointerTypeWithoutDataAndIncompatibleType(t *testing.T) {
	source := struct {
		Foo *SourceTypeA
	}{}
	dest := struct {
		Foo struct {
			Baz string
		}
	}{}
	source.Foo = nil

	r := testMapper.Map(&source, &dest)

	assert.NotEmpty(t, r.Errors)
}

func TestErrorWhenUsingIncompatibleTypes(t *testing.T) {
	source := struct{ Foo string }{}
	dest := struct{ Foo int }{}
	r := testMapper.Map(&source, &dest)

	assert.NotEmpty(t, r.Errors)
}

func TestDefaultInt32ToIntConversion(t *testing.T) {
	source := struct{ Foo int32 }{9}
	dest := struct{ Foo int }{}
	r := testMapper.Map(&source, &dest)

	assert.Empty(t, r.Errors)
	assert.Equal(t, 9, dest.Foo)
}

func TestDefaultIntToInt32Conversion(t *testing.T) {
	source := struct{ Foo int }{9}
	dest := struct{ Foo int32 }{}
	r := testMapper.Map(&source, &dest)

	assert.Empty(t, r.Errors)
	assert.Equal(t, int32(9), dest.Foo)
}

func TestDefaultUIntToInt32Conversion(t *testing.T) {
	source := struct{ Foo uint }{9}
	dest := struct{ Foo int32 }{}
	r := testMapper.Map(&source, &dest)

	assert.Empty(t, r.Errors)
	assert.Equal(t, int32(9), dest.Foo)
}

func TestDefaultFloat64ToFloat32Conversion(t *testing.T) {
	source := struct{ Foo float64 }{9.1}
	dest := struct{ Foo float32 }{}
	r := testMapper.Map(&source, &dest)

	assert.Empty(t, r.Errors)
	assert.InDelta(t, float32(9.1), dest.Foo, 0.0001)
}

func TestDefaultFloat32ToFloat64Conversion(t *testing.T) {
	source := struct{ Foo float32 }{9.1}
	dest := struct{ Foo float64 }{}
	r := testMapper.Map(&source, &dest)

	assert.Empty(t, r.Errors)
	assert.InDelta(t, float64(9.1), dest.Foo, 0.0001)
}

func TestWithLooseOption(t *testing.T) {
	source := struct {
		Foo string
		Baz int
	}{"Foo", 42}
	dest := struct {
		Foo string
		Bar int
	}{}
	r := testMapper.Map(&source, &dest)
	assert.Equal(t, dest.Foo, "Foo")
	assert.Equal(t, dest.Bar, 0)
	assert.NotEmpty(t, r.Errors)
}

type Model struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func TestSourceComposedStruct(t *testing.T) {
	type structWithModel struct {
		Model
		Name string
	}
	source := structWithModel{
		Model: Model{
			ID:        1,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			DeletedAt: nil,
		},
		Name: "Test",
	}
	type dto struct {
		ID        uint
		CreatedAt time.Time
		UpdatedAt time.Time
		DeletedAt *time.Time
		Name      string
	}
	dest := &dto{}
	r := &Result{}
	m := Mapper{IgnoreDestFields: []string{"DestTypeA", "Model"}}
	assert.NotPanics(t, func() {
		r = m.Map(source, dest)
	})
	assert.Empty(t, r.Errors)
	assert.Equal(t, source.ID, dest.ID)
	assert.Equal(t, source.CreatedAt, dest.CreatedAt)
	assert.Equal(t, source.UpdatedAt, dest.UpdatedAt)
	assert.Equal(t, source.Name, dest.Name)
}

func TestDestComposedStruct(t *testing.T) {
	source := struct {
		A     int32
		B     uint
		Frank []SourceTypeA
	}{5, 6, []SourceTypeA{{Foo: 1, Bar: "bar"}}}
	dest := struct {
		Model
		DestTypeA
		A     int
		B     uint64
		Frank []DestTypeA
	}{}

	m := Mapper{IgnoreDestFields: []string{"DestTypeA", "Model"}}
	r := m.Map(&source, &dest)
	assert.Equal(t, 5, dest.A)
	assert.Equal(t, uint64(6), dest.B)
	assert.NotEmpty(t, dest.Frank)
	assert.Equal(t, 1, dest.Frank[0].Foo)
	assert.Equal(t, "bar", dest.Frank[0].Bar)
	assert.Empty(t, r.Errors)
}

type SourceParent struct {
	Children []SourceTypeA
}

type DestParent struct {
	Children []DestTypeA
}

type SourceTypeA struct {
	Foo int
	Bar string
}

type DestTypeA struct {
	Foo int
	Bar string
}
