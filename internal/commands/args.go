package commands

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// InputFieldType defines the type of input field
type InputFieldType int

const (
	InputTypeText    InputFieldType = iota // Free text input
	InputTypeNumber                        // Integer input
	InputTypeBoolean                       // Boolean/checkbox
	InputTypeSelect                        // Dropdown selection
)

// InputField defines a parameter for a command (generated from struct tags)
type InputField struct {
	Name        string         // Field name from "form" tag
	Label       string         // Display label from "title" tag
	Type        InputFieldType // Field type (inferred from Go type or "type" tag)
	Required    bool           // Whether field is required (!optional)
	Default     interface{}    // Default value from "default" tag
	Placeholder string         // Placeholder text
	Validation  string         // Validation rules from "validate" tag
}

// GenerateInputFields reads struct tags and creates InputField slice
// Struct tags format:
//
//	Field type `form:"name" title:"Display" type:"input|select|confirm" validate:"rules" default:"val" optional:"true"`
func GenerateInputFields(argsStruct interface{}) ([]InputField, error) {
	if argsStruct == nil {
		return nil, nil
	}

	val := reflect.ValueOf(argsStruct)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("argsStruct must be a struct or pointer to struct")
	}

	typ := val.Type()
	fields := []InputField{}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Get form tag (required)
		formTag := field.Tag.Get("form")
		if formTag == "" {
			continue // Skip fields without form tag
		}

		// Get title tag (required)
		titleTag := field.Tag.Get("title")
		if titleTag == "" {
			titleTag = field.Name // Fallback to field name
		}

		// Determine field type
		fieldType := inferFieldType(field.Type, field.Tag.Get("type"))

		// Check if optional (default is required)
		optional := field.Tag.Get("optional") == "true"
		required := !optional

		// Get default value
		defaultVal := field.Tag.Get("default")

		// Get validation rules
		validation := field.Tag.Get("validate")

		inputField := InputField{
			Name:       formTag,
			Label:      titleTag,
			Type:       fieldType,
			Required:   required,
			Default:    defaultVal,
			Validation: validation,
		}

		fields = append(fields, inputField)
	}

	return fields, nil
}

// inferFieldType determines InputFieldType from Go type and tag
func inferFieldType(goType reflect.Type, typeTag string) InputFieldType {
	// Explicit type tag takes precedence
	switch typeTag {
	case "input":
		return InputTypeText
	case "number":
		return InputTypeNumber
	case "select":
		return InputTypeSelect
	case "confirm":
		return InputTypeBoolean
	}

	// Infer from Go type
	switch goType.Kind() {
	case reflect.Bool:
		return InputTypeBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return InputTypeNumber
	case reflect.String:
		return InputTypeText
	default:
		return InputTypeText
	}
}

// ParseInlineArgs populates struct from positional arg string
// Format: "value1 value2 value3" maps to struct fields in order
// Optional fields use defaults if not provided
func ParseInlineArgs(argsStruct interface{}, argString string) error {
	if argsStruct == nil {
		return nil
	}

	val := reflect.ValueOf(argsStruct)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("argsStruct must be a pointer to struct")
	}
	val = val.Elem()

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("argsStruct must be a pointer to struct")
	}

	// Split args string by whitespace
	argString = strings.TrimSpace(argString)
	var args []string
	if argString != "" {
		args = strings.Fields(argString)
	}

	typ := val.Type()
	argIdx := 0

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Skip fields without form tag
		formTag := field.Tag.Get("form")
		if formTag == "" {
			continue
		}

		optional := field.Tag.Get("optional") == "true"
		defaultTag := field.Tag.Get("default")

		var argValue string
		if argIdx < len(args) {
			// Use provided arg
			argValue = args[argIdx]
			argIdx++
		} else if optional && defaultTag != "" {
			// Use default for optional field
			argValue = defaultTag
		} else if optional {
			// Optional field without default, skip
			continue
		} else {
			// Required field missing
			titleTag := field.Tag.Get("title")
			if titleTag == "" {
				titleTag = field.Name
			}
			return fmt.Errorf("missing required argument: %s", titleTag)
		}

		// Convert and set value based on field type
		if err := setFieldValue(fieldVal, argValue); err != nil {
			titleTag := field.Tag.Get("title")
			if titleTag == "" {
				titleTag = field.Name
			}
			return fmt.Errorf("invalid value for %s: %w", titleTag, err)
		}

		// Validate if validation tag present
		validation := field.Tag.Get("validate")
		if validation != "" {
			if err := validateField(fieldVal, validation); err != nil {
				titleTag := field.Tag.Get("title")
				if titleTag == "" {
					titleTag = field.Name
				}
				return fmt.Errorf("validation failed for %s: %w", titleTag, err)
			}
		}
	}

	return nil
}

// setFieldValue sets a reflect.Value from a string
func setFieldValue(fieldVal reflect.Value, value string) error {
	if !fieldVal.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("must be an integer")
		}
		fieldVal.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a positive integer")
		}
		fieldVal.SetUint(uintVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("must be true or false")
		}
		fieldVal.SetBool(boolVal)
	default:
		return fmt.Errorf("unsupported field type: %s", fieldVal.Kind())
	}

	return nil
}

// validateField validates a field value against validation rules
func validateField(fieldVal reflect.Value, validation string) error {
	rules := strings.Split(validation, ",")

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)

		if rule == "required" {
			// Check if zero value
			if fieldVal.IsZero() {
				return fmt.Errorf("required")
			}
		} else if strings.HasPrefix(rule, "min=") {
			minStr := strings.TrimPrefix(rule, "min=")
			min, err := strconv.ParseInt(minStr, 10, 64)
			if err != nil {
				continue
			}

			switch fieldVal.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if fieldVal.Int() < min {
					return fmt.Errorf("must be >= %d", min)
				}
			}
		} else if strings.HasPrefix(rule, "max=") {
			maxStr := strings.TrimPrefix(rule, "max=")
			max, err := strconv.ParseInt(maxStr, 10, 64)
			if err != nil {
				continue
			}

			switch fieldVal.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if fieldVal.Int() > max {
					return fmt.Errorf("must be <= %d", max)
				}
			}
		}
		// Add more validation rules as needed (portmap, etc.)
	}

	return nil
}
