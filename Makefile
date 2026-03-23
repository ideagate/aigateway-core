.PHONY: proto-generate

## proto-generate: lint proto files then generate Go stubs from .proto definitions
proto-generate:
	buf lint
	buf generate

