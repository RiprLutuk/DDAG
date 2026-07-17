package gateway

import (
	"strings"

	"github.com/ddag/ddag/internal/models"
	"go.yaml.in/yaml/v2"
)

// OpenAPISpec is the OpenAPI 3.0 document generated from DDAG API metadata.
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       OpenAPIInfo            `json:"info"`
	Paths      map[string]OpenAPIPath `json:"paths"`
	Components OpenAPIComponents      `json:"components"`
	Tags       []OpenAPITag           `json:"tags,omitempty"`
}

type OpenAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type OpenAPIPath map[string]OpenAPIOperation

type OpenAPIOperation struct {
	Summary     string                     `json:"summary,omitempty"`
	Description string                     `json:"description,omitempty"`
	Tags        []string                   `json:"tags,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses"`
	Security    []map[string][]string      `json:"security,omitempty"`
	DDAGMethod  string                     `json:"x-ddag-http-method,omitempty"`
}

type OpenAPIParameter struct {
	Name     string        `json:"name"`
	In       string        `json:"in"`
	Required bool          `json:"required"`
	Schema   OpenAPISchema `json:"schema"`
}

type OpenAPIRequestBody struct {
	Required bool                        `json:"required,omitempty"`
	Content  map[string]OpenAPIMediaType `json:"content"`
}

type OpenAPIMediaType struct {
	Schema OpenAPISchema `json:"schema"`
}

type OpenAPIResponse struct {
	Description string                      `json:"description"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

type OpenAPIComponents struct {
	SecuritySchemes map[string]OpenAPISecurityScheme `json:"securitySchemes"`
}

type OpenAPISecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
}

type OpenAPISchema struct {
	Type                 string                   `json:"type,omitempty"`
	Format               string                   `json:"format,omitempty"`
	Properties           map[string]OpenAPISchema `json:"properties,omitempty"`
	AdditionalProperties any                      `json:"additionalProperties,omitempty"`
	Items                *OpenAPISchema           `json:"items,omitempty"`
}

type OpenAPITag struct {
	Name string `json:"name"`
}

// GenerateOpenAPISpec builds an OpenAPI 3.0 spec from published API metadata.
func GenerateOpenAPISpec(apis []models.APIDefinition) OpenAPISpec {
	spec := OpenAPISpec{
		OpenAPI: "3.0.3",
		Info:    OpenAPIInfo{Title: "DDAG Dynamic APIs", Version: "1.0.0"},
		Paths:   map[string]OpenAPIPath{},
		Components: OpenAPIComponents{SecuritySchemes: map[string]OpenAPISecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
		}},
	}
	seenTags := map[string]bool{}
	for _, api := range apis {
		method := strings.ToLower(api.Method)
		if method == "" {
			method = "get"
		}
		// OpenAPI 3.0 supports only its defined operation keys. QUERY (RFC 10008)
		// is represented as POST for tooling compatibility and marked explicitly.
		isQuery := method == "query"
		if isQuery {
			method = "post"
		}
		if spec.Paths[api.Path] == nil {
			spec.Paths[api.Path] = OpenAPIPath{}
		}
		op := OpenAPIOperation{
			Summary:     api.Name,
			Description: api.Description,
			Tags:        []string{api.Namespace},
			DDAGMethod: func() string {
				if isQuery {
					return "QUERY"
				}
				return ""
			}(),
			Responses: map[string]OpenAPIResponse{
				"200": {
					Description: "Successful DDAG response",
					Content: map[string]OpenAPIMediaType{
						"application/json": {Schema: genericSuccessSchema()},
					},
				},
				"401": {Description: "Unauthorized"},
				"403": {Description: "Forbidden"},
			},
		}
		if api.RequiredScope != "" {
			op.Security = []map[string][]string{{"bearerAuth": {api.RequiredScope}}}
		}
		for _, p := range api.Parameters {
			param := OpenAPIParameter{
				Name:     p.Name,
				In:       parameterLocation(p.Source),
				Required: p.Required || p.Source == "path",
				Schema:   schemaForParamType(p.ParamType),
			}
			if p.Source == "body" {
				if op.RequestBody == nil {
					op.RequestBody = &OpenAPIRequestBody{
						Content: map[string]OpenAPIMediaType{
							"application/json": {Schema: OpenAPISchema{Type: "object", Properties: map[string]OpenAPISchema{}}},
						},
					}
				}
				op.RequestBody.Required = op.RequestBody.Required || p.Required
				op.RequestBody.Content["application/json"].Schema.Properties[p.Name] = param.Schema
				continue
			}
			op.Parameters = append(op.Parameters, param)
		}
		spec.Paths[api.Path][method] = op
		if api.Namespace != "" && !seenTags[api.Namespace] {
			seenTags[api.Namespace] = true
			spec.Tags = append(spec.Tags, OpenAPITag{Name: api.Namespace})
		}
	}
	return spec
}

func GenerateOpenAPIYAML(apis []models.APIDefinition) ([]byte, error) {
	return yaml.Marshal(GenerateOpenAPISpec(apis))
}

func parameterLocation(source string) string {
	switch source {
	case "path", "query", "header":
		return source
	default:
		return "query"
	}
}

func schemaForParamType(t string) OpenAPISchema {
	switch t {
	case "int":
		return OpenAPISchema{Type: "integer", Format: "int64"}
	case "number":
		return OpenAPISchema{Type: "number", Format: "double"}
	case "bool":
		return OpenAPISchema{Type: "boolean"}
	case "uuid":
		return OpenAPISchema{Type: "string", Format: "uuid"}
	case "date":
		return OpenAPISchema{Type: "string", Format: "date"}
	default:
		return OpenAPISchema{Type: "string"}
	}
}

func genericSuccessSchema() OpenAPISchema {
	return OpenAPISchema{
		Type: "object",
		Properties: map[string]OpenAPISchema{
			"success":    {Type: "boolean"},
			"request_id": {Type: "string"},
			"row_count":  {Type: "integer"},
			"rows": {
				Type: "array",
				Items: &OpenAPISchema{
					Type: "object",
				},
			},
		},
	}
}
