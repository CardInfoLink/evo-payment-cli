// Package registry provides API metadata loading and management.
package registry

// Registry holds the loaded API metadata from meta_data.json.
type Registry struct {
	Version  string    `json:"version"`
	Services []Service `json:"services"`
}

// Service represents a top-level API service (e.g., payment, linkpay).
type Service struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Resources   map[string]*Resource `json:"resources"`
}

// Resource represents a group of related API methods under a service.
type Resource struct {
	Methods map[string]*Method `json:"methods"`
}

// Method represents a single API endpoint.
type Method struct {
	HTTPMethod  string                `json:"httpMethod"`
	Path        string                `json:"path"`
	Description string                `json:"description"`
	Parameters  map[string]*Parameter `json:"parameters,omitempty"`
	RequestBody map[string]*BodyField `json:"requestBody,omitempty"`
}

// Parameter defines an API parameter (path or query).
type Parameter struct {
	Location   string   `json:"location"` // "path" or "query"
	Required   bool     `json:"required"`
	Type       string   `json:"type"`
	FromConfig string   `json:"fromConfig,omitempty"`
	Enum       []string `json:"enum,omitempty"`
}

// BodyField defines a field in the request body.
type BodyField struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
}
