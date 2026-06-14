.PHONY: build test scan serve clean

build:
	go build -o drift ./cmd/drift

test:
	go test ./...

scan:
	go run ./cmd/drift scan --state-file testdata/aws/terraform.tfstate --provider aws --region us-east-1 --output rich --no-color

scan-unmanaged:
	go run ./cmd/drift scan --state-file testdata/aws/terraform.tfstate --provider aws --region us-east-1 --detect-unmanaged --output json

serve:
	go run ./cmd/drift --config configs/examples/drift.yaml serve

clean:
	rm -f drift drift.db
