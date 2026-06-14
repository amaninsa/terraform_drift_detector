.PHONY: build test scan serve clean

build:
	go build -o drift ./cmd/drift

test:
	go test ./...

scan:
	go run ./cmd/drift scan --state-file testdata/aws/terraform.tfstate --provider aws --region us-east-1 --output table

serve:
	go run ./cmd/drift serve --port 8080

clean:
	rm -f drift drift.db
