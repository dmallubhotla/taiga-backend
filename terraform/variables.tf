variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "app_name" {
  description = "Application name used for resource naming"
  type        = string
  default     = "taiga"
}

variable "app_port" {
  description = "Port the application listens on"
  type        = number
  default     = 8080
}

variable "tuffas_applier_role_arn" {
  type        = string
  description = "IAM role ARN for Terraform to assume when applying changes"
}


# Database variables
variable "db_name" {
  description = "Database name"
  type        = string
  default     = "trygo"
}

variable "db_username" {
  description = "Database username"
  type        = string
  default     = "trygo"
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "db_allocated_storage" {
  description = "Initial database storage size in GB"
  type        = number
  default     = 20
}

variable "db_max_allocated_storage" {
  description = "Maximum database storage size in GB for autoscaling"
  type        = number
  default     = 50
}
#
# # ECS variables
# variable "ecs_cpu" {
#   description = "CPU units for ECS task (1024 = 1 vCPU)"
#   type        = number
#   default     = 256
# }
#
# variable "ecs_memory" {
#   description = "Memory for ECS task in MB"
#   type        = number
#   default     = 512
# }
