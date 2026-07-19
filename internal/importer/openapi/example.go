package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v4"

	"github.com/yusupkhemraev/payk/internal/core"
)

// maxExampleDepth caps schema recursion so circular $refs cannot loop.
const maxExampleDepth = 6

// attachBody generates a JSON example body from the operation's request
// body schema.
func (b *builder) attachBody(req *core.Request, op *v3.Operation) {
	if op.RequestBody == nil || op.RequestBody.Content == nil {
		return
	}

	media := pickJSONMedia(op.RequestBody.Content.FromOldest())
	if media == nil {
		for contentType := range op.RequestBody.Content.FromOldest() {
			b.warn("%s: body media type %q imported without example", req.Name, contentType)
			break
		}
		return
	}

	value := exampleFromMedia(media)
	if value == nil {
		return
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return
	}
	req.Body = core.Body{Type: "json", Content: string(pretty)}
}

func pickJSONMedia(seq func(func(string, *v3.MediaType) bool)) *v3.MediaType {
	for contentType, media := range seq {
		if strings.Contains(contentType, "json") {
			return media
		}
	}
	return nil
}

func exampleFromMedia(media *v3.MediaType) any {
	if v := decodeNode(media.Example); v != nil {
		return v
	}
	if media.Examples != nil {
		for _, ex := range media.Examples.FromOldest() {
			if v := decodeNode(ex.Value); v != nil {
				return v
			}
		}
	}
	return exampleFromSchema(media.Schema, 0)
}

// exampleFromSchema builds a representative value for a schema: explicit
// example/default/enum first, then type-driven synthesis.
func exampleFromSchema(proxy *base.SchemaProxy, depth int) any {
	if proxy == nil || depth > maxExampleDepth {
		return nil
	}
	schema := proxy.Schema()
	if schema == nil {
		return nil
	}

	if v := decodeNode(schema.Example); v != nil {
		return v
	}
	if v := decodeNode(schema.Default); v != nil {
		return v
	}
	if len(schema.Enum) > 0 {
		if v := decodeNode(schema.Enum[0]); v != nil {
			return v
		}
	}

	// Composition keywords: take the first branch.
	for _, branch := range [][]*base.SchemaProxy{schema.AllOf, schema.OneOf, schema.AnyOf} {
		if len(branch) > 0 {
			if v := exampleFromSchema(branch[0], depth+1); v != nil {
				return v
			}
		}
	}

	switch schemaType(schema) {
	case "object":
		obj := map[string]any{}
		if schema.Properties != nil {
			for name, prop := range schema.Properties.FromOldest() {
				if v := exampleFromSchema(prop, depth+1); v != nil {
					obj[name] = v
				} else {
					obj[name] = nil
				}
			}
		}
		return obj
	case "array":
		if schema.Items != nil && schema.Items.IsA() {
			if item := exampleFromSchema(schema.Items.A, depth+1); item != nil {
				return []any{item}
			}
		}
		return []any{}
	case "string":
		return stringExample(schema.Format)
	case "integer":
		return 0
	case "number":
		return 0
	case "boolean":
		return true
	}
	return nil
}

// schemaType returns the effective type; in 3.1 Type is a list, and object
// is implied by the presence of properties.
func schemaType(schema *base.Schema) string {
	for _, t := range schema.Type {
		if t != "null" {
			return t
		}
	}
	if schema.Properties != nil && schema.Properties.Len() > 0 {
		return "object"
	}
	return ""
}

func stringExample(format string) string {
	switch format {
	case "date-time":
		return "2024-01-01T00:00:00Z"
	case "date":
		return "2024-01-01"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "uri", "url":
		return "https://example.com"
	default:
		return "string"
	}
}

func decodeNode(node *yaml.Node) any {
	if node == nil {
		return nil
	}
	var v any
	if err := node.Decode(&v); err != nil {
		return nil
	}
	return v
}

// paramExample renders a scalar example for a parameter value.
func paramExample(param *v3.Parameter) string {
	if v := decodeNode(param.Example); v != nil {
		return scalarString(v)
	}
	if v := exampleFromSchema(param.Schema, 0); v != nil {
		return scalarString(v)
	}
	return ""
}

func scalarString(v any) string {
	switch v.(type) {
	case map[string]any, []any:
		return ""
	}
	return fmt.Sprintf("%v", v)
}
