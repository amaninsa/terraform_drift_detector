# Terraform Drift Detector

An advanced, cloud-agnostic platform that continuously compares **Terraform state** against **live cloud infrastructure** to identify configuration drift — without `terraform plan` or `terraform apply`.

```
 ██████╗ ██████╗ ██╗███████╗████████╗
 ██╔══██╗██╔══██╗██║██╔════╝╚══██╔══╝
 ██║  ██║██████╔╝██║█████╗     ██║
 Terraform Drift Detector — infrastructure drift, zero terraform plan
```

## Features

| Feature | Description |
|---------|-------------|
| **Multi-cloud** | AWS, Azure, and GCP adapters |
| **State backends** | Local files and S3 remote state |
| **Drift types** | Missing, modified, tag-only, unmanaged, fetch errors |
| **Unmanaged detection** | Find cloud resources not in Terraform state |
| **Scheduler** | In-process cron scheduling via config |
| **Webhooks** | Slack/HTTP notifications on drift |
| **Rich CLI** | Colorized output, spinners, summary boxes |
| **Dashboard** | Web UI with on-demand scan trigger |
| **REST API** | List scans, trigger scans, export JSON |

## Quick Start

```bash
make build

# Rich CLI output (default)
./drift scan --state-file ./terraform.tfstate --provider aws --region us-east-1

# S3 remote state + unmanaged detection
./drift scan \
  --state-bucket my-tf-state \
  --state-key prod/terraform.tfstate \
  --state-region us-east-1 \
  --provider aws --region us-east-1 \
  --detect-unmanaged \
  --persist

# Config-driven scan
./drift --config configs/examples/drift.yaml scan --workspace prod

# Start server + scheduler + dashboard
./drift --config configs/examples/drift.yaml serve
```

## CLI Commands

```bash
drift scan          # Run drift scan (output: rich, json, table)
drift report list   # List persisted scans
drift report show   # Show scan JSON
drift serve         # API + dashboard + scheduler
drift workspaces    # List configured workspaces
drift schedule list # List cron schedules
drift version       # Version info
```

### Scan flags

| Flag | Description |
|------|-------------|
| `--state-file` | Local Terraform state path |
| `--state-bucket` / `--state-key` | S3 remote state |
| `--config` | YAML config file |
| `--workspace` | Workspace name from config |
| `--provider` | `aws`, `azure`, `gcp` |
| `--detect-unmanaged` | Find resources not in state |
| `--output rich` | Colorized table output (default) |
| `--persist` | Save to SQLite |
| `--no-color` | Plain output |

## Supported Resources

| Provider | Resource Types |
|----------|----------------|
| **AWS** | `aws_instance`, `aws_s3_bucket`, `aws_security_group`, `aws_vpc`, `aws_subnet` |
| **Azure** | `azurerm_resource_group`, `azurerm_virtual_network`, `azurerm_subnet`, `azurerm_storage_account` |
| **GCP** | `google_compute_instance`, `google_storage_bucket`, `google_compute_network` |

## Configuration

```yaml
server:
  port: 8080
  database: ./drift.db

webhooks:
  - url: https://hooks.slack.com/services/...
    on_drift: true

workspaces:
  - name: prod
    provider: aws
    region: us-east-1
    detect_unmanaged: true
    state:
      type: s3
      bucket: my-tf-state
      key: prod/terraform.tfstate
      region: us-east-1

schedules:
  - workspace: prod
    cron: "0 */6 * * *"
```

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/scans` | List scans |
| POST | `/api/v1/scans` | Trigger scan `{"workspace":"prod"}` |
| GET | `/api/v1/scans/{id}` | Full report |
| GET | `/api/v1/workspaces` | List workspaces |

## Architecture

```
drift.yaml ──► Config ──► Scheduler (cron)
                │
Terraform State ──► State Loader (local/S3) ──► Scan Runner
                                                      │
                                              Cloud Adapter (AWS/Azure/GCP)
                                                      │
                                              Normalizer ──► Diff Engine
                                                      │
                                    ┌─────────────────┼─────────────────┐
                                    ▼                 ▼                 ▼
                                  CLI (rich)        REST API         Webhooks
```

## Development

```bash
go test ./...
make build
make scan    # Demo scan against fixture state
```

## License

See [LICENSE](LICENSE).
