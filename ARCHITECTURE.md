# Terraform Drift Detector — Architecture

This document describes the **High-Level Design (HLD)** and **Low-Level Design (LLD)** for the Terraform Drift Detector platform.

---

## 1. Design Philosophy

The system follows a **read-only reconciliation** pattern:

```
Expected (Terraform State)  vs  Actual (Cloud APIs)  →  Normalized Diff  →  Drift Report
```

It deliberately avoids `terraform plan` / `terraform apply`. Instead it:

1. Treats Terraform state as the **source of truth for intent**
2. Fetches **live metadata** from cloud provider APIs
3. Normalizes both sides into a **canonical resource model**
4. Produces structured drift with no mutation of infrastructure

---

## 2. High-Level Design (HLD)

### 2.1 System Context (C4 — Level 1)

```mermaid
flowchart TB
    subgraph actors [Actors]
        SRE[SRE / Platform Engineer]
        CI[CI/CD Pipeline]
        CRON[Scheduler]
    end

    subgraph system [Terraform Drift Detector]
        DRIFT[Drift Platform]
    end

    subgraph external [External Systems]
        TFSTATE[(Terraform State<br/>Local / S3)]
        AWS[AWS APIs]
        AZURE[Azure ARM APIs]
        GCP[GCP APIs]
        HOOKS[Webhooks<br/>Slack / PagerDuty]
        DB[(SQLite Store)]
    end

    SRE -->|CLI rich/json| DRIFT
    CI -->|JSON output| DRIFT
    CRON -->|cron schedules| DRIFT
    SRE -->|Dashboard| DRIFT

    DRIFT -->|read| TFSTATE
    DRIFT -->|Describe/List| AWS
    DRIFT -->|Get/List| AZURE
    DRIFT -->|Get/List| GCP
    DRIFT -->|persist scans| DB
    DRIFT -->|POST on drift| HOOKS
```

### 2.2 Container Diagram (C4 — Level 2)

```mermaid
flowchart TB
    subgraph presentation [Presentation Layer]
        CLI[CLI<br/>cobra + lipgloss]
        API[REST API]
        WEB[Web Dashboard<br/>static HTML/JS]
    end

    subgraph orchestration [Orchestration Layer]
        CFG[Config Loader<br/>drift.yaml]
        SCHED[Scheduler<br/>robfig/cron]
        SCAN[Scan Runner]
    end

    subgraph core [Core Engine]
        STATE[State Parser<br/>TF v4 JSON]
        BACKEND[State Backend<br/>local / S3]
        MAP[Resource Mapper]
        NORM[Normalizer]
        DIFF[Diff Engine]
    end

    subgraph cloud [Cloud Abstraction]
        FACTORY[Adapter Factory]
        AWS_A[AWS Adapter]
        AZ_A[Azure Adapter]
        GCP_A[GCP Adapter]
    end

    subgraph persistence [Persistence and Notify]
        STORE[SQLite Store]
        NOTIFY[Webhook Client]
    end

    CLI --> CFG
    CLI --> SCAN
    API --> SCAN
    API --> STORE
    WEB --> API
    SCHED --> SCAN

    SCAN --> BACKEND
    BACKEND --> STATE
    SCAN --> MAP
    SCAN --> FACTORY
    FACTORY --> AWS_A
    FACTORY --> AZ_A
    FACTORY --> GCP_A
    SCAN --> NORM
    SCAN --> DIFF
    SCAN --> NOTIFY
    SCAN --> STORE
```

### 2.3 Logical Data Flow

```mermaid
flowchart LR
    A[Terraform State] --> B[Extract Managed Resources]
    B --> C[Map TF Type to Cloud Fetch]
    C --> D[Parallel Cloud Fetch]
    D --> E[Normalize Expected and Actual]
    E --> F[Deep Compare]
    F --> G{Detect Unmanaged?}
    G -->|yes| H[List Cloud Resources]
    H --> I[Diff vs State IDs]
    G -->|no| J[Drift Report]
    I --> J
    J --> K[CLI / API / Webhook / DB]
```

### 2.4 Deployment Topology

```mermaid
flowchart TB
    subgraph mode1 [Mode 1: CLI Only]
        BIN[drift binary]
        BIN --> TF1[(local.tfstate)]
        BIN --> AWS1[AWS creds]
    end

    subgraph mode2 [Mode 2: Server Mode]
        SERVE[drift serve]
        SERVE --> DB2[(drift.db)]
        SERVE --> WEB2[Dashboard :8080]
        SERVE --> CRON2[In-process Scheduler]
        CRON2 --> S3[(S3 State)]
        SERVE --> WH[Webhooks]
    end

    subgraph mode3 [Mode 3: CI Integration]
        CI_JOB[CI Job]
        CI_JOB -->|drift scan --output json| GATE{Drifted > 0?}
        GATE -->|yes| FAIL[Fail Pipeline]
        GATE -->|no| PASS[Pass]
    end
```

---

## 3. Low-Level Design (LLD)

### 3.1 Package Architecture

```
cmd/drift/                    Entry point
├── internal/
│   ├── cli/                  Commands, rich output, UI
│   ├── config/               drift.yaml loader
│   ├── models/               Domain types (Resource, DriftReport)
│   ├── state/                TF state v4 parser
│   │   └── backend/          local + S3 loaders
│   ├── mapper/               TF type → fetch strategy registry
│   ├── cloudtypes/           Adapter interface + MockAdapter
│   ├── cloud/                Adapter factory (aws/azure/gcp)
│   ├── providers/
│   │   ├── aws/              EC2, S3 SDK
│   │   ├── azure/            ARM SDK
│   │   └── gcp/              Compute + Storage SDK
│   ├── normalize/            Attribute filtering and coercion
│   ├── diff/                 Comparison and classification
│   ├── scan/                 Scan orchestration
│   ├── scheduler/            Cron job runner
│   ├── notify/               Webhook dispatcher
│   ├── store/                SQLite persistence
│   └── api/                  HTTP handlers
└── web/                      Dashboard static assets
```

### 3.2 Core Interface — Cloud Adapter

The extensibility boundary lives in `internal/cloudtypes`:

```go
type Adapter interface {
    Name() string
    FetchResource(ctx, ref) (*Resource, error)
    FetchResources(ctx, refs) (map[string]*Resource, error)   // parallel
    ListResources(ctx, types, opts) ([]*Resource, error)        // unmanaged
}
```

| Method | Purpose |
|--------|---------|
| `FetchResources` | Point lookups for state-managed resources |
| `ListResources` | Inventory scan for unmanaged detection |

Factory (`internal/cloud`) selects the adapter by provider:

| Provider | SDK |
|----------|-----|
| `aws` | AWS SDK (EC2, S3) |
| `azure` | ARM SDK (Resources, Network, Storage) |
| `gcp` | Google API (Compute, Storage) |

### 3.3 Resource Mapping Registry

Each Terraform type maps to a fetch strategy in `internal/mapper/registry.go`:

```yaml
aws_instance:
  provider: aws
  id_attribute: id
  compare_keys: [instance_type, ami, subnet_id, vpc_security_group_ids]
```

Flow:

```
state.ManagedResource
    → mapper.ToResourceRef()           # extract cloud ID
    → adapter.FetchResources()         # live fetch
    → normalize.NormalizeResource()    # both sides
    → diff.CompareResources()          # classify drift
```

### 3.4 Canonical Domain Model

```mermaid
classDiagram
    class Resource {
        +string ID
        +string Address
        +string Provider
        +string Type
        +string CloudID
        +map Attributes
        +map Tags
        +string Source
    }

    class DriftItem {
        +string Address
        +DriftType DriftType
        +Resource Expected
        +Resource Actual
        +[]FieldDiff Diff
        +map TagsDiff
    }

    class DriftReport {
        +string ScanID
        +string Workspace
        +string StateSource
        +DriftSummary Summary
        +[]DriftItem Items
    }

    class DriftType {
        <<enumeration>>
        missing
        modified
        tag_only
        unmanaged
        in_sync
        fetch_error
    }

    DriftReport "1" --> "*" DriftItem
    DriftItem --> DriftType
    DriftItem --> Resource : expected
    DriftItem --> Resource : actual
```

### 3.5 Scan Sequence (On-Demand)

```mermaid
sequenceDiagram
    actor User
    participant CLI
    participant Runner as Scan Runner
    participant Loader as State Backend
    participant Parser as State Parser
    participant Mapper
    participant Adapter as Cloud Adapter
    participant Norm as Normalizer
    participant Diff as Diff Engine
    participant Store as SQLite
    participant Hook as Webhook

    User->>CLI: drift scan --state-file ...
    CLI->>Runner: Run(ctx, opts)
    Runner->>Loader: Load(state source)
    Loader->>Parser: TF state v4 JSON
    Parser-->>Runner: []ManagedResource

    loop each managed resource
        Runner->>Mapper: Get(type)
        Mapper-->>Runner: Mapping + ResourceRef
    end

    Runner->>Adapter: FetchResources(refs) parallel
    Adapter-->>Runner: map[address]Resource

    loop each target
        Runner->>Norm: Normalize(expected, actual)
        Runner->>Diff: CompareResources()
        Diff-->>Runner: DriftItem
    end

    opt detect_unmanaged
        Runner->>Adapter: ListResources(types)
        Adapter-->>Runner: cloud inventory
        Runner->>Diff: UnmanagedItem() for unknown IDs
    end

    Runner-->>CLI: DriftReport
    CLI->>Store: SaveReport (if --persist)
    CLI->>Hook: NotifyDrift (if configured)
    CLI-->>User: rich / json / table output
```

### 3.6 Scheduled Scan Sequence

```mermaid
sequenceDiagram
    participant Cron as robfig/cron
    participant Sched as Scheduler
    participant Runner as Scan Runner
    participant Store as SQLite
    participant Hook as Webhook

    Note over Cron: "0 */6 * * *"
    Cron->>Sched: runScheduled(workspace)
    Sched->>Runner: Run(ctx, workspace opts)
    Runner-->>Sched: DriftReport
    Sched->>Store: SaveReport
    alt summary.drifted > 0
        Sched->>Hook: POST drift payload
    end
```

### 3.7 Normalization Pipeline

Computed and read-only Terraform attributes are stripped before comparison:

```
Raw State Attributes
    → filter by compare_keys (per resource type)
    → apply DefaultIgnoreRules (arn, tags_all, id, ...)
    → coerce types (float→int, sort slices)
    → strip tags (compared separately)
    → Normalized Resource
```

Tag comparison is isolated so drift can be classified as `tag_only` vs `modified`.

### 3.8 Drift Classification Logic

```mermaid
flowchart TD
    START[Compare Expected vs Actual] --> FOUND{Actual exists?}
    FOUND -->|no| MISSING[missing]
    FOUND -->|yes| ATTR{Attribute diffs?}
    ATTR -->|yes| TAGS{Tag diffs?}
    ATTR -->|no| TAGS2{Tag diffs?}
    TAGS -->|yes| MOD[modified]
    TAGS -->|no| MOD2[modified]
    TAGS2 -->|yes| TAG[tag_only]
    TAGS2 -->|no| SYNC[in_sync]

    LIST[ListResources] --> UNMANAGED{Cloud ID in state?}
    UNMANAGED -->|no| UNM[unmanaged]
```

---

## 4. Component Deep Dive

### 4.1 State Ingestion Layer

| Backend | Implementation | Source |
|---------|----------------|--------|
| Local | `state.LoadFromFile()` | `--state-file` |
| S3 | AWS S3 `GetObject` | `--state-bucket` + `--state-key` |

Parser rules:

- Only `mode: managed` resources
- Skips `data.*` sources
- Resolves provider aliases (`registry.terraform.io/hashicorp/aws` → `aws`)
- Builds addresses including module prefixes

### 4.2 Cloud Adapter Layer

| Provider | SDK | Fetch | List (unmanaged) |
|----------|-----|-------|------------------|
| AWS | aws-sdk-go-v2 | DescribeInstances, HeadBucket, DescribeSecurityGroups | Describe*, ListBuckets |
| Azure | azure-sdk-for-go | ResourceGroups.Get, VNets.Get | List Pagers |
| GCP | cloud.google.com/go | Instances.Get, Buckets.Attrs | Buckets iterator, Networks.List |

Auth uses standard credential chains (env vars, IAM roles, workload identity, `azidentity`, ADC).

### 4.3 Supported Resource Types

| Provider | Terraform Types |
|----------|-----------------|
| AWS | `aws_instance`, `aws_s3_bucket`, `aws_security_group`, `aws_vpc`, `aws_subnet` |
| Azure | `azurerm_resource_group`, `azurerm_virtual_network`, `azurerm_subnet`, `azurerm_storage_account` |
| GCP | `google_compute_instance`, `google_storage_bucket`, `google_compute_network` |

### 4.4 Persistence Layer

SQLite schema (simplified):

```sql
CREATE TABLE scans (
    scan_id       TEXT PRIMARY KEY,
    workspace     TEXT NOT NULL,
    state_source  TEXT NOT NULL,
    provider      TEXT NOT NULL,
    region        TEXT,
    started_at    TEXT NOT NULL,
    finished_at   TEXT NOT NULL,
    duration      TEXT NOT NULL,
    summary_json  TEXT NOT NULL,
    report_json   TEXT NOT NULL
);
```

Enables scan history, dashboard trends, and `drift report list/show`.

### 4.5 Notification Layer

Webhook payload on drift:

```json
{
  "event": "drift.detected",
  "scan_id": "...",
  "workspace": "prod",
  "summary": { "drifted": 3, "missing": 1, "modified": 2 },
  "drifted_items": []
}
```

Fires only when `summary.drifted > 0` and `on_drift: true` in config.

---

## 5. API Surface

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/scans` | List scan metadata |
| POST | `/api/v1/scans` | Trigger workspace scan |
| GET | `/api/v1/scans/{id}` | Full drift report |
| GET | `/api/v1/scans/{id}/json` | JSON download |
| GET | `/api/v1/workspaces` | Configured workspaces |
| GET | `/` | Dashboard static files |

---

## 6. Configuration Model

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

drift:
  detect_unmanaged: false
  concurrency: 10
  ignore_rules:
    - tags_all
```

See [`configs/examples/drift.yaml`](configs/examples/drift.yaml) for a full example.

---

## 7. Extensibility Model

```mermaid
flowchart LR
    subgraph add_provider [Add New Provider]
        A1[Implement cloudtypes.Adapter]
        A2[Register in cloud/factory.go]
        A3[Add mappings in mapper/registry.go]
        A4[Add ignore rules in normalize]
    end

    subgraph add_resource [Add Resource Type]
        B1[Add mapping entry]
        B2[Implement fetchOne + list method]
        B3[Define compare_keys]
    end

    subgraph add_backend [Add State Backend]
        C1[Extend backend.Loader]
        C2[Add case in Load switch]
    end
```

No changes to the diff engine or CLI are required for new resource types — only mapping and adapter methods.

---

## 8. Non-Functional Characteristics

| Concern | Approach |
|---------|----------|
| **Performance** | Parallel `FetchResources` per resource (goroutines) |
| **Safety** | Read-only cloud API calls; no mutations |
| **Portability** | Single Go binary; SQLite embedded |
| **Observability** | Structured JSON reports; webhook events |
| **False positives** | Default ignore lists + per-type `compare_keys` |
| **Partial failure** | Per-resource `fetch_error`; scan continues |
| **Multi-tenancy** | Workspace abstraction in config (v1: single-process) |

---

## 9. Architecture Comparison

| Capability | `terraform plan` | Terraform Cloud Drift | Drift Detector |
|------------|------------------|----------------------|----------------|
| Needs TF binary | Yes | Yes (managed) | **No** |
| Needs provider plugins | Yes | Yes | **No** |
| Multi-cloud single tool | Via TF | Via TF | **Native adapters** |
| Unmanaged detection | No | Limited | **Yes** |
| Custom scheduling | Manual | TFC schedules | **Built-in cron** |
| CI-friendly JSON | Plan output | API | **Native JSON/rich CLI** |

---

## 10. Future Architecture (Roadmap)

```mermaid
flowchart TB
    subgraph current [Current v1.1]
        C1[Local + S3 state]
        C2[AWS / Azure / GCP]
        C3[SQLite + single process]
    end

    subgraph future [Future v2]
        F1[GCS / Azure Blob / TFC state]
        F2[go-plugin external providers]
        F3[PostgreSQL + multi-tenant API]
        F4[Incremental scans + caching]
        F5[SARIF export for CI gates]
        F6[K8s Operator / CronJob]
    end

    current --> future
```

---

## 11. One-Page Summary

```
┌─────────────────────────────────────────────────────────────────────┐
│                     TERRAFORM DRIFT DETECTOR                        │
├─────────────────────────────────────────────────────────────────────┤
│  INPUTS          │  CORE PIPELINE           │  OUTPUTS              │
│  ───────         │  ─────────────           │  ───────              │
│  TF State v4     │  Parse → Map → Fetch     │  Rich CLI             │
│  (local/S3)      │  → Normalize → Diff      │  JSON / Table         │
│  drift.yaml      │  → Classify → Report       │  REST API             │
│  Cloud creds     │                            │  Dashboard            │
│                  │                            │  Webhooks             │
│                  │                            │  SQLite history       │
├─────────────────────────────────────────────────────────────────────┤
│  CLOUDS: AWS · Azure · GCP    │    DRIFT: missing · modified ·     │
│                               │    tag_only · unmanaged · in_sync   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Related Documentation

- [README.md](README.md) — Quick start and usage
- [configs/examples/drift.yaml](configs/examples/drift.yaml) — Configuration reference
