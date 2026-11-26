package provider

import (
	"context"
	"testing"
)

func TestYamlDocumentSemanticEquals(t *testing.T) {
	cases := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{
			name:  "identical",
			a:     "outer:\n  key: value\n",
			b:     "outer:\n  key: value\n",
			equal: true,
		},
		{
			// What the deployment values service actually does: same keys,
			// same bytes, different order.
			name:  "reordered keys",
			a:     "a: one\nb: true\n",
			b:     "b: true\na: one\n",
			equal: true,
		},
		{
			name:  "reordered nested keys",
			a:     "outer:\n  a: one\n  b: two\n",
			b:     "outer:\n  b: two\n  a: one\n",
			equal: true,
		},
		{
			name:  "changed value",
			a:     "a: one\n",
			b:     "a: two\n",
			equal: false,
		},
		{
			name:  "removed key",
			a:     "a: one\nb: true\n",
			b:     "a: one\n",
			equal: false,
		},
		{
			// List order is meaningful, unlike mapping key order.
			name:  "reordered list entries",
			a:     "items:\n  - x\n  - y\n",
			b:     "items:\n  - y\n  - x\n",
			equal: false,
		},
		{
			name:  "removed list entry",
			a:     "items:\n  - x\n  - y\n",
			b:     "items:\n  - x\n",
			equal: false,
		},
		{
			name:  "unparseable falls back to string compare",
			a:     "\tnot: [valid",
			b:     "\tnot: [valid",
			equal: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := newYamlDocumentValue(tc.a).StringSemanticEquals(
				context.Background(), newYamlDocumentValue(tc.b))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tc.equal {
				t.Errorf("semantic equality = %v, want %v", got, tc.equal)
			}
		})
	}
}
