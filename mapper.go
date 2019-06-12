package mapper

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Mapper holds config.
type Mapper struct {
	PanicOnMissingField      bool
	PanicOnIncompatibleTypes bool
	FieldNameMaps            map[string]string
	IgnoreCase               bool
	SourceTag                string
	DestTag                  string
	FuzzyMatchFieldNames     bool
	CustomMappers            []CustomFieldMapper
	IgnoreDestFields         []string
}

// CustomFieldMapper is the function signature for custom mappers.
type CustomFieldMapper func(sourceVal reflect.Value, sourceType reflect.Type, destVal reflect.Value, destType reflect.Type) (handled bool)

// DefaultMapper hold default configuration for basic mapping
var DefaultMapper = Mapper{
	PanicOnIncompatibleTypes: true,
	PanicOnMissingField:      true,
}

// Result is the returned data from the Map() function.
type Result struct {
	MissingSourceFields []string
	Errors              []error
	scope               []string
}

// Map calls Map on the default mapper.
func Map(source, dest interface{}) *Result {
	return DefaultMapper.Map(source, dest)
}

// Map tries to map fields from the source in the destination that are similar.
// It applies basic configuration, conversion and custom mappers if ther are any.
func (m *Mapper) Map(source, dest interface{}) *Result {
	result := &Result{}
	var destType = reflect.TypeOf(dest)
	if destType.Kind() != reflect.Ptr {
		panic("Dest must be a pointer type")
	}
	var sourceVal = reflect.ValueOf(source)
	var destVal = reflect.ValueOf(dest).Elem()
	m.mapValues(result, sourceVal, destVal)
	if len(result.MissingSourceFields) > 0 {
		errMsg := "Missing fields from source struct: " + strings.Join(result.MissingSourceFields, ", ")
		if m.PanicOnMissingField {
			panic(errMsg)
		}
		result.addError(errMsg)
	}
	return result
}

func (r *Result) addError(errMsg string) {
	r.Errors = append(r.Errors, errors.New(errMsg))
}

// Error returns a list of errors found while processing the map, if not set to panic
func (r *Result) Error() error {
	if len(r.Errors) == 0 {
		return nil
	}
	if len(r.Errors) == 1 {
		return r.Errors[0]
	}
	if len(r.Errors) == 2 {
		return errors.New(r.Errors[0].Error() + " and " + r.Errors[1].Error())
	}
	msg := r.Errors[0].Error()
	for i := 1; i < len(r.Errors)-1; i++ {
		msg = msg + ", " + r.Errors[i].Error()
	}
	msg = msg + ", and " + r.Errors[len(r.Errors)-1].Error()
	return errors.New(msg)
}

// mapValues maps field values from a source struct to a destination struct.
func (m *Mapper) mapValues(result *Result, sourceVal, destVal reflect.Value) {
	destType := destVal.Type()

	if destType.Kind() == reflect.Struct {
		// dereference source pointer structs
		if sourceVal.Type().Kind() == reflect.Ptr {
			// if the source is nil, create a new instance of it so we can copy the blank fields over.
			// I believe if you skip this and return immediately it would be fine,
			// you'd just not give the custom mapper a chance to run on nil types.
			if sourceVal.IsNil() {
				sourceVal = reflect.New(sourceVal.Type().Elem())
			}
			sourceVal = sourceVal.Elem()
		}
	}

	sourceType := sourceVal.Type()

	// check for custom field mappers
	for _, mapper := range m.CustomMappers {
		if handled := mapper(sourceVal, sourceType, destVal, destType); handled {
			// return if the mapper has handled the conversion for us
			return
		}
	}

	// if type is a pointer, create a new object of that type and recursively call mapValues
	if destType.Kind() == reflect.Ptr {
		if valueIsNil(sourceVal) {
			return
		}
		val := reflect.New(destType.Elem())
		m.mapValues(result, sourceVal, val.Elem())
		destVal.Set(val)
		return
	}

	// DO NOT COPY THIS EXAMPLE without reading the rest of the comment here.
	// TRY TO AVOID custom type conversions hard-coded into the mapper like this.
	// This is suitable for type conversions between types that are part of the standard library and very common. If this is not applicable to your new case, then instead write a custom mapper; see CustomFieldMapper.
	//
	// If both times are time.Time, do a special conversion.
	if destType.Kind() == reflect.Struct {
		if destVal.Type().PkgPath() == "time" && destVal.Type().Name() == "Time" {
			if sourceType.Kind() == reflect.Struct {
				if sourceVal.Type().PkgPath() == "time" && sourceVal.Type().Name() == "Time" {
					destVal.Set(sourceVal)
					return
				}
			}
		}
	}

	// if destination type is a struct, iterate dest struct's fields and map the field
	if destType.Kind() == reflect.Struct {
		for i := 0; i < destVal.NumField(); i++ {
			result.scope = append(result.scope, destType.Field(i).Name)
			m.mapField(result, sourceVal, destVal, i)

			// NOTE: I think these scope checks are no longer necessary. Since implementing the result object, Map should be goroutine-safe as it has no internal state.
			if len(result.scope) == 0 {
				panic("Scope length is unexpectedly zero. You may be sharing a mapper across goroutines. Use one mapper per goroutine or lock access to it")
			}
			if len(result.scope) > 200 {
				panic("scope stack is too large. You may be sharing a mapper across goroutines. Use one mapper per goroutine or lock access to it")
			}
			result.scope = result.scope[0 : len(result.scope)-1]
		}
		return
	}

	// handle all int types at once
	destIsInt, _ := isIntType(destVal)
	sourceIsInt, sourceIntVal := isIntType(sourceVal)
	if sourceIsInt && destIsInt {
		destVal.SetInt(sourceIntVal)
		return
	}

	// handle all uint types at once
	destIsUint, _ := isUintType(destVal)
	sourceIsUint, sourceUintVal := isUintType(sourceVal)
	if sourceIsUint && destIsUint {
		destVal.SetUint(sourceUintVal)
		return
	}

	if destIsInt && sourceIsUint {
		destVal.SetInt(int64(sourceUintVal))
		return
	}

	destIsFloat, _ := isFloatType(destVal)
	sourceIsFloat, sourceFloatVal := isFloatType(sourceVal)
	if sourceIsFloat && destIsFloat {
		destVal.SetFloat(sourceFloatVal)
		return
	}

	// if the types match, copy the value
	if destType == sourceVal.Type() {
		destVal.Set(sourceVal)
		return
	}

	// for slices, map the slice.
	if destType.Kind() == reflect.Slice {
		m.mapSlice(result, sourceVal, destVal)
		return
	}

	errMsg := fmt.Sprintf("Currently not supported (source %s -> dest %s), write a custom mapper for this. see CustomFieldMapper.", sourceVal.Type(), destVal.Type())
	if m.PanicOnIncompatibleTypes {
		panic(errMsg)
	}
	result.addError(errMsg)
}

// mapField maps a specific field on a struct type from source to destination.
func (m *Mapper) mapField(result *Result, source, destVal reflect.Value, i int) {
	destType := destVal.Type()

	// catch any type-conversion panic so we can add some context to it.
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Error mapping field: %s. DestType: %v. SourceType: %v. Error: %v", destType.Field(i).Name, destType, source.Type(), r)
			if m.PanicOnIncompatibleTypes {
				panic(errMsg)
			}
			result.addError(errMsg)
		}
	}()

	destField := destVal.Field(i)
	sourceField := m.findSourceField(result, source, destType.Field(i))
	if !sourceField.IsValid() {
		return
	}
	m.mapValues(result, sourceField, destField)
}

// findSourceField finds the matching field on the source object.
func (m *Mapper) findSourceField(result *Result, source reflect.Value, destFieldType reflect.StructField) reflect.Value {
	destFieldName := m.formattedDestFieldName(destFieldType)
	if m.includes(m.IgnoreDestFields, destFieldName) {
		return reflect.Value{}
	}
	for i := 0; i < source.NumField(); i++ {
		sourceFieldName := m.formattedSourceFieldName(source.Type().Field(i))
		if sourceFieldName == destFieldName {
			return source.Field(i)
		}
	}

	// if we didn't find the field, try looking in anonymous composed structs.
	for i := 0; i < source.NumField(); i++ {
		sourceField := source.Type().Field(i)
		if sourceField.Anonymous && sourceField.Type.Kind() == reflect.Struct {
			// probe anonymous structs recursively, but discard the result, since not finding the field in _this_ struct is not evidence that it's missing, per se. It'd have to be missing from all composed structs and the parent struct to truly be missing.
			sourceFieldFound := m.findSourceField(&Result{}, source.Field(i), destFieldType)
			if sourceFieldFound.IsValid() {
				return sourceFieldFound
			}
		}
	}

	result.MissingSourceFields = append(result.MissingSourceFields, result.scopedFieldName())
	return reflect.Value{}
}

// includes returns true if `str` is in the list of `strings`, considering support for fuzzy matches.
func (m *Mapper) includes(strings []string, str string) bool {
	str = m.fuzzy(str)
	for _, s := range strings {
		if m.fuzzy(s) == str {
			return true
		}
	}
	return false
}

// scopedFieldName returns the path to the current field, regardless of where we are in the recursive call structure.
// eg: student.contact.phone_number
func (r *Result) scopedFieldName() string {
	return strings.Join(r.scope, ".")
}

// formattedDestFieldName is the name of the field in the destination object, with configuration options taken into account
func (m *Mapper) formattedDestFieldName(f reflect.StructField) string {
	fieldName := f.Name
	if len(m.DestTag) > 0 {
		if f, ok := f.Tag.Lookup(m.DestTag); ok {
			fieldName = strings.Split(f, ",")[0]
		}
	}
	fieldName = m.fuzzy(fieldName)
	for k, v := range m.FieldNameMaps {
		if m.fuzzy(v) == fieldName {
			return m.fuzzy(k)
		}
	}
	return fieldName
}

// formattedSourceFieldName is the name of the field in the source object, with certain configuration options taken into account
func (m *Mapper) formattedSourceFieldName(f reflect.StructField) string {
	fieldName := f.Name
	if len(m.SourceTag) > 0 {
		if f, ok := f.Tag.Lookup(m.SourceTag); ok {
			fieldName = strings.Split(f, ",")[0]
		}
	}
	return m.fuzzy(fieldName)
}

// fuzzy returns a lowercased version of a field name with underscores removed, depending on configuration, for fuzzy fieldname matching.
// If these are not configured it returns the field name as-is.
func (m *Mapper) fuzzy(f string) string {
	if m.FuzzyMatchFieldNames {
		f = strings.ToLower(strings.Replace(f, "_", "", -1))
	} else if m.IgnoreCase {
		f = strings.ToLower(f)
	}
	return f
}

// valueIsNil is like reflect.ValueOf(x).IsNil() except it won't panic if the value is not a pointer, it just returns false.
func valueIsNil(value reflect.Value) bool {
	return value.Type().Kind() == reflect.Ptr && value.IsNil()
}

// mapSlice maps a slice from source to dest, creating elements as necessary.
func (m *Mapper) mapSlice(result *Result, sourceVal, destVal reflect.Value) {
	destType := destVal.Type()
	length := sourceVal.Len()
	target := reflect.MakeSlice(destType, length, length)
	for j := 0; j < length; j++ {
		val := reflect.New(destType.Elem()).Elem()
		m.mapValues(result, sourceVal.Index(j), val)
		target.Index(j).Set(val)
	}
	if length == 0 {
		m.checkArrayTypesAreCompatible(result, sourceVal, destVal)
	}
	destVal.Set(target)
}

// checkArrayTypesAreCompatible is used when mapping empty slices. The idea being that if the types are incompatible it
// should not quietly succeed in tests and then later fail in production where the array is not empty.
// Instead create dummy objects and try to map it to see if there are errors.
// Discards objects afterwards, but keeps the errors.
func (m *Mapper) checkArrayTypesAreCompatible(result *Result, sourceVal, destVal reflect.Value) {
	dummyDest := reflect.New(reflect.PtrTo(destVal.Type()))
	dummySource := reflect.MakeSlice(sourceVal.Type(), 1, 1)
	m.mapValues(result, dummySource, dummyDest.Elem())
}

// isIntType returns true if the type is an int type and returns the int64 value of it.
func isIntType(x reflect.Value) (bool, int64) {
	switch x.Type().Name() {
	case "int":
		return true, int64(x.Interface().(int))
	case "int8":
		return true, int64(x.Interface().(int8))
	case "int16":
		return true, int64(x.Interface().(int16))
	case "int32":
		return true, int64(x.Interface().(int32))
	case "int64":
		return true, x.Interface().(int64)
	}
	return false, 0
}

// isUintType returns true if the type is an int type and returns the uint64 value of it.
func isUintType(x reflect.Value) (bool, uint64) {
	switch x.Type().Name() {
	case "uint":
		return true, uint64(x.Interface().(uint))
	case "uint8":
		return true, uint64(x.Interface().(uint8))
	case "uint16":
		return true, uint64(x.Interface().(uint16))
	case "uint32":
		return true, uint64(x.Interface().(uint32))
	case "uint64":
		return true, x.Interface().(uint64)
	}
	return false, 0
}

// isFloatType returns true if the type is a float type and returns the float64 value of it.
func isFloatType(x reflect.Value) (bool, float64) {
	switch x.Type().Name() {
	case "float32":
		return true, float64(x.Interface().(float32))
	case "float64":
		return true, x.Interface().(float64)
	}
	return false, 0
}
