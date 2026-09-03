package core

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
)

// ValidateConfigOptionContract applies the machine-readable portion of an
// adapter contract without constructing the adapter. Custom validators still
// own cross-field and platform-specific semantics.
func ValidateConfigOptionContract(owner string, options []ConfigOption, values map[string]any) error {
	for _, rawOption := range options {
		option := FinalizeConfigOption(rawOption)
		if option.Internal {
			continue
		}
		value, configured := values[option.Key]
		provided := configured && configValueProvided(value)
		if option.Requirement == ConfigRequirementRequired && !provided {
			return fmt.Errorf("%s: option %q is required", owner, option.Key)
		}
		if !provided {
			continue
		}
		if !configValueMatchesType(value, option.Type) {
			return fmt.Errorf("%s: option %q must be %s, got %T", owner, option.Key, option.Type, value)
		}
		if configTypeIncludes(option.Type, "table") {
			if key, child, ok := firstNonStringTableEntry(value); ok {
				return fmt.Errorf("%s: option %q entry %q must be string, got %T", owner, option.Key, key, child)
			}
		}
		if option.ClosedValues {
			if stringValue, ok := value.(string); ok && !configContractContains(option.Values, stringValue) {
				return fmt.Errorf("%s: option %q must be one of %s, got %q", owner, option.Key, strings.Join(option.Values, ", "), stringValue)
			}
		}
		numeric, numericValue := configNumericValue(value)
		if !numericValue {
			continue
		}
		if option.Minimum != nil && numeric < *option.Minimum {
			return fmt.Errorf("%s: option %q must be >= %s%s", owner, option.Key, formatConfigContractNumber(*option.Minimum), configUnitSuffix(option.Unit))
		}
		if option.Maximum != nil && numeric > *option.Maximum {
			return fmt.Errorf("%s: option %q must be <= %s%s", owner, option.Key, formatConfigContractNumber(*option.Maximum), configUnitSuffix(option.Unit))
		}
	}
	return nil
}

func configTypeIncludes(typeName, target string) bool {
	for _, candidate := range strings.Split(typeName, "|") {
		candidate = strings.TrimSpace(candidate)
		if idx := strings.Index(candidate, " "); idx >= 0 {
			candidate = candidate[:idx]
		}
		if candidate == target {
			return true
		}
	}
	return false
}

func firstNonStringTableEntry(value any) (string, any, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return "", nil, false
	}
	entries := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		entries[iter.Key().String()] = iter.Value().Interface()
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		child := entries[key]
		if _, ok := child.(string); !ok {
			return key, child, true
		}
	}
	return "", nil, false
}

func configContractContains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func configValueProvided(value any) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() > 0
	default:
		return true
	}
}

func configValueMatchesType(value any, typeName string) bool {
	for _, candidate := range strings.Split(typeName, "|") {
		candidate = strings.TrimSpace(candidate)
		if idx := strings.Index(candidate, " "); idx >= 0 {
			candidate = candidate[:idx]
		}
		switch candidate {
		case "string":
			if _, ok := value.(string); ok {
				return true
			}
		case "boolean", "bool":
			if _, ok := value.(bool); ok {
				return true
			}
		case "integer":
			if numeric, ok := configNumericValue(value); ok && math.Trunc(numeric) == numeric {
				return true
			}
		case "number":
			if _, ok := configNumericValue(value); ok {
				return true
			}
		case "string[]":
			if configStringSlice(value) {
				return true
			}
		case "table":
			rv := reflect.ValueOf(value)
			if rv.IsValid() && rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
				return true
			}
		}
	}
	return false
}

func configStringSlice(value any) bool {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || (rv.Kind() != reflect.Array && rv.Kind() != reflect.Slice) {
		return false
	}
	for i := 0; i < rv.Len(); i++ {
		if _, ok := rv.Index(i).Interface().(string); !ok {
			return false
		}
	}
	return true
}

func configNumericValue(value any) (float64, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return 0, false
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	default:
		return 0, false
	}
}

func configUnitSuffix(unit string) string {
	if unit == "" {
		return ""
	}
	return " " + unit
}
