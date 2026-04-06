default: build

build:
	go build -o terraform-provider-garage

generate:
	cd internal/garage && go generate ./...

test:
	go test ./... -v -count=1 -race

testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 30m

testfilter:
	TF_ACC=1 go test ./... -v -count=1 -run=$(FILTER)

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

docs:
	tfplugindocs generate --provider-name garage

docs-validate:
	tfplugindocs validate --provider-name garage

.PHONY: build generate test testacc testfilter lint fmt docs docs-validate
