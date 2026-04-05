output "repository_uri" {
  description = "ECR Public repository URI — use as the image base in docker push."
  value       = aws_ecrpublic_repository.app.repository_uri
}

output "github_role_arn" {
  description = "IAM role ARN to set as the AWS_ROLE_ARN secret in GitHub."
  value       = aws_iam_role.github_ecr_push.arn
}
