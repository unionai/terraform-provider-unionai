package provider

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"gopkg.in/yaml.v3"
)

// yamlDocumentType is a string that compares by YAML structure rather than by
// text. The deployment values service re-serializes the document it is given --
// same keys and same bytes, different key order -- so a byte comparison reports
// a change on every plan even when nothing was edited.
type yamlDocumentType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = yamlDocumentType{}

func (t yamlDocumentType) Equal(o attr.Type) bool {
	other, ok := o.(yamlDocumentType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t yamlDocumentType) String() string { return "yamlDocumentType" }

func (t yamlDocumentType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return yamlDocumentValue{StringValue: in}, nil
}

func (t yamlDocumentType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", attrValue)
	}
	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}
	return stringValuable, nil
}

func (t yamlDocumentType) ValueType(context.Context) attr.Value {
	return yamlDocumentValue{}
}

type yamlDocumentValue struct {
	basetypes.StringValue
}

var _ basetypes.StringValuableWithSemanticEquals = yamlDocumentValue{}

func (v yamlDocumentValue) Type(context.Context) attr.Type { return yamlDocumentType{} }

func (v yamlDocumentValue) Equal(o attr.Value) bool {
	other, ok := o.(yamlDocumentValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals treats two documents as equal when they parse to the
// same structure, so key reordering by the server is not reported as drift.
// Unparseable input falls back to string equality rather than erroring, so a
// malformed document still reaches the server and produces its error.
func (v yamlDocumentValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	newValue, ok := newValuable.(yamlDocumentValue)
	if !ok {
		diags.AddError("Semantic Equality Check Error", fmt.Sprintf("unexpected value type %T", newValuable))
		return false, diags
	}
	var a, b any
	if err := yaml.Unmarshal([]byte(v.ValueString()), &a); err != nil {
		return v.ValueString() == newValue.ValueString(), diags
	}
	if err := yaml.Unmarshal([]byte(newValue.ValueString()), &b); err != nil {
		return v.ValueString() == newValue.ValueString(), diags
	}
	return reflect.DeepEqual(a, b), diags
}

func newYamlDocumentValue(s string) yamlDocumentValue {
	return yamlDocumentValue{StringValue: basetypes.NewStringValue(s)}
}
