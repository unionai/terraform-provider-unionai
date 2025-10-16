package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
