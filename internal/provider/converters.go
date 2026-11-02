package provider

import (
	"fmt"
	"math"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// int64ToUint32 narrows a Terraform number to a uint32 wire field that must be at least minimum.
// A null or unknown value is 0. The second result describes a value out of range, or is empty.
func int64ToUint32(attribute string, value types.Int64, minimum int64) (uint32, string) {
	if value.IsNull() || value.IsUnknown() {
		return 0, ""
	}
	number := value.ValueInt64()
	if number < minimum || number > math.MaxUint32 {
		return 0, fmt.Sprintf("%s must be between %d and %d, got %d.", attribute, minimum, uint32(math.MaxUint32), number)
	}
	return uint32(number), ""
}

func convertSetToStrings(input types.Set) []string {
	output := make([]string, 0, len(input.Elements()))
	for _, item := range input.Elements() {
		output = append(output, item.(types.String).ValueString())
	}
	return output
}

func convertStringsToSet(input []string) types.Set {
	output := make([]attr.Value, 0, len(input))
	for _, item := range input {
		output = append(output, types.StringValue(item))
	}
	return types.SetValueMust(types.StringType, output)
}

func convertArrayToSetGetter[T any](input []T, getter func(T) string) types.Set {
	output := make([]attr.Value, 0, len(input))
	for _, item := range input {
		output = append(output, types.StringValue(getter(item)))
	}
	return types.SetValueMust(types.StringType, output)
}
