data "garage_buckets" "all" {}

output "bucket_ids" {
  value = data.garage_buckets.all.buckets[*].id
}
