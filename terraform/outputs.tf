# output "database_url" {
#   description = "PostgreSQL connection URL"
#   value       = "postgres://${var.db_username}:${var.db_password}@${aws_db_instance.main.address}:5432/${var.db_name}?sslmode=require"
#   sensitive   = true
# }
#
# output "database_host" {
#   description = "RDS instance hostname"
#   value       = aws_db_instance.main.address
# }
#
# output "database_port" {
#   description = "RDS instance port"
#   value       = aws_db_instance.main.port
# }

output "s3_bucket_name" {
  description = "S3 bucket name for file storage"
  value       = aws_s3_bucket.app_files.bucket
}

output "s3_bucket_arn" {
  description = "S3 bucket ARN"
  value       = aws_s3_bucket.app_files.arn
}

# output "ecr_repository_url" {
#   description = "ECR repository URL for container images"
#   value       = aws_ecr_repository.app.repository_url
# }
#
# output "ecs_cluster_name" {
#   description = "ECS cluster name"
#   value       = aws_ecs_cluster.main.name
# }
#
# output "ecs_service_name" {
#   description = "ECS service name"
#   value       = aws_ecs_service.app.name
# }
#
# output "cloudwatch_log_group" {
#   description = "CloudWatch log group name"
#   value       = aws_cloudwatch_log_group.app.name
# }
#
# output "ecs_task_execution_role_arn" {
#   description = "ECS task execution role ARN"
#   value       = aws_iam_role.ecs_task_execution.arn
# }
#
# output "ecs_task_role_arn" {
#   description = "ECS task role ARN"
#   value       = aws_iam_role.ecs_task.arn
# }

# Configuration for your Go application
output "app_config" {
  description = "Environment variables for the Go application"
  value = {
    TAIGA_APP_PORT        = var.app_port
    TAIGA_APP_ENVIRONMENT = "production"
    # TAIGA_DB_DRIVER               = "postgres"
    # TAIGA_DB_HOST                 = aws_db_instance.main.address
    # TAIGA_DB_PORT                 = "5432"
    # TAIGA_DB_NAME                 = var.db_name
    # TAIGA_DB_USER                 = var.db_username
    # TAIGA_DB_SSL_MODE             = "require"
    TAIGA_FILE_REPO_TYPE          = "s3"
    TAIGA_FILE_REPO_S3_BUCKET     = aws_s3_bucket.app_files.bucket
    TAIGA_FILE_REPO_S3_REGION     = var.aws_region
    TAIGA_FILE_REPO_S3_PREFIX     = "files"
    TAIGA_FILE_REPO_PREFIX_LENGTH = "2"
  }
  sensitive = false
}
