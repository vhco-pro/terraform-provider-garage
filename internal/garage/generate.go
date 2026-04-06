package garage

//go:generate python3 openapi/fix_spec.py openapi/spec.json openapi/spec.processed.json
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml openapi/spec.processed.json
