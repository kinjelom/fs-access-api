package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPIYAML []byte

//go:embed index.html
var IndexHTML []byte

//go:embed redoc.html
var RedocHTML []byte

//go:embed swagger.html
var SwaggerHTML []byte
