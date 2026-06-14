# Terraform Drift Detector

A cloud-agnostic platform that continuously compares **Terraform state files** against **live cloud infrastructure** to identify configuration drift — without running `terraform plan` or `terraform apply`.

## Features

- **State parsing** — Reads Terraform state v4 JSON (local files; remote backends planned)
- **Cloud comparison** — Fetches live resource metadata from cloud provider APIs
- **Normalization** — Maps both sides into a common model with configurable ignore rules
- **Drift detection** — Identifies missing resources, modified attributes, and tag changes
- **Multiple interfaces** — CLI (JSON/table), REST API, and web dashboard
- **Persistence** — SQLite storage for scan history and reporting

## Supported Providers (v1)

| Provider | Resource Types |
|----------|----------------|
| AWS | `aws_instance`, `aws_s3_bucket`, `aws_security_group`, `aws_vpc`, `aws_subnet` |

## Quick Start

### Build

```bash
go build -o drift ./cmd/drift
```

### Run a Scan

```bash
./drift scan \
  --state-file ./testdata/aws/terraform.tfstate \
  --provider aws \
  --region us-east-1 \
  --output json
```

Use a mock-free scan against real AWS by configuring credentials (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, or instance profile) and pointing at your state file.

### Persist Results & View Dashboard

```bash
./drift scan --state-file ./terraform.tfstate --provider aws --region us-east-1 --persist
./drift serve --port 8080 --db drift.db
```

Open [http://localhost:8080](http://localhost:8080) for the dashboard.

### List Reports (CLI)

```bash
./drift report list
./drift report show <scan-id>
```

## Architecture

```
Terraform State ──► State Parser ──► Resource Mapper ──► Cloud Adapter
                                                              │
                                                              ▼
                                                         Normalizer
                                                              │
                         Drift Report ◄── Diff Engine ◄───────┘
                              │
                    ┌─────────┼─────────┐
                    ▼         ▼         ▼
                  CLI       API     Dashboard
```

## Drift Types

| Type | Description |
|------|-------------|
| `missing` | Resource in state but not found in cloud |
| `modified` | Attributes differ from state |
| `tag_only` | Only tags differ |
| `in_sync` | Matches Terraform state |
| `fetch_error` | Could not fetch or unsupported type |

## Example JSON Output

```json
{
  "scan_id": "…",
  "workspace": "default",
  "summary": {
    "total_resources": 3,
    "drifted": 1,
    "modified": 1,
    "in_sync": 2
  },
  "items": [
    {
      "address": "aws_instance.web",
      "drift_type": "modified",
      "diff": [
        {
          "path": "instance_type",
          "expected": "t3.micro",
          "actual": "t3.small"
        }
      ]
    }
  ]
}
```

## Configuration

See [`configs/examples/drift.yaml`](configs/examples/drift.yaml) for workspace and schedule examples.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/scans` | List recent scans |
| GET | `/api/v1/scans/{id}` | Full drift report |
| GET | `/api/v1/scans/{id}/json` | JSON download |

## Project Layout

```
cmd/drift/           CLI entrypoint
internal/
  state/             Terraform state parser
  mapper/            Resource type → cloud fetch mapping
  providers/         Cloud adapters (AWS, mock)
  normalize/         Attribute normalization
  diff/              Drift comparison engine
  scan/              Scan orchestration
  store/             SQLite persistence
  api/               REST API
  cli/               CLI commands
web/                 Dashboard static files
testdata/            Fixture state files
```

## Extending

1. Add a mapping entry in `internal/mapper/registry.go`
2. Implement fetch logic in the provider adapter
3. Add ignore rules for computed attributes in `internal/normalize/normalize.go`

## Development

```bash
go test ./...
```

## License

See [LICENSE](LICENSE).
