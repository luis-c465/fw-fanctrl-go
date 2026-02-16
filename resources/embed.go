package resources

import _ "embed"

var (
	//go:embed config.json
	DefaultConfigJSON []byte

	//go:embed config.schema.json
	ConfigSchemaJSON []byte
)
