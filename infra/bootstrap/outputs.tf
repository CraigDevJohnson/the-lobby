output "state_bucket_name" {
  description = "Bucket name to use in every root's S3 backend."
  value       = aws_s3_bucket.state.bucket
}

output "state_bucket_arn" {
  value = aws_s3_bucket.state.arn
}
