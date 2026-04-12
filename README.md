# terraform-provider-garage

Terraform provider for [Garage](https://garagehq.deuxfleurs.fr/), the self-hosted S3-compatible distributed object storage engine. Manages buckets, access keys, permissions, and cluster layout through the Garage admin API (v2).

## Setup

```hcl
terraform {
  required_providers {
    garage = {
      source = "vhco-pro/garage"
    }
  }
}

provider "garage" {
  endpoint = "http://garage.local:3903"
  token    = var.garage_admin_token
}
```

The endpoint and token can also be set via `GARAGE_ENDPOINT` and `GARAGE_TOKEN` environment variables.

## Resources

- `garage_bucket` — S3 bucket
- `garage_key` — API access key
- `garage_bucket_alias` — global or local bucket alias
- `garage_bucket_permission` — key-to-bucket permission grant
- `garage_layout_node` — cluster layout node assignment
- `garage_admin_token` — admin API token

## Data Sources

- `garage_bucket` / `garage_buckets`
- `garage_key` / `garage_keys`
- `garage_cluster_status` / `garage_cluster_health` / `garage_cluster_layout`
- `garage_node_info`
- `garage_admin_token` / `garage_admin_tokens`

## Development

Requires a running Garage instance for acceptance tests. The included docker-compose file sets one up:

```shell
docker compose -f testdata/docker-compose.yml up -d

# unit tests
go test ./... -v

# acceptance tests
TF_ACC=1 GARAGE_ENDPOINT=http://localhost:3903 GARAGE_TOKEN=test-admin-token \
  go test ./... -v -timeout 30m
```

See the [registry docs](https://registry.terraform.io/providers/vhco-pro/garage/latest/docs) for full resource and data source documentation.
