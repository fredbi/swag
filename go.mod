module github.com/go-openapi/swag

retract v0.24.0 // bad tagging of the main module: superseeded by v0.24.1

replace (
	github.com/go-openapi/swag/conv => ./conv
	github.com/go-openapi/swag/jsonutils => ./jsonutils
	github.com/go-openapi/swag/jsonutils/fixtures_test => ./jsonutils/fixtures_test
	github.com/go-openapi/swag/loading => ./loading
	github.com/go-openapi/swag/stringutils => ./stringutils
	github.com/go-openapi/swag/typeutils => ./typeutils
	github.com/go-openapi/swag/yamlutils => ./yamlutils
)

go 1.25.0
