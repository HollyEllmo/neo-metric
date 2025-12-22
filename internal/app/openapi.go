package app

import (
	_ "embed"
)

//go:embed openapi.yaml
var OpenAPISpec []byte
