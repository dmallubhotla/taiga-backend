terraform {
  required_version = ">= 1.2"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  backend "s3" {
    region       = "us-east-2"
    bucket       = "hrudayme-test-tfstate"
    key          = "taiga-tf"
    use_lockfile = true
    assume_role  = { role_arn = "arn:aws:iam::677425296084:role/tfstate_backend_role" }
  }
}

locals {
  common_tags = {
    Project     = var.app_name
    Environment = "test"
    ManagedBy   = "terraform"
  }
}

provider "aws" {
  region = var.aws_region
  assume_role {
    role_arn = var.tuffas_applier_role_arn
  }
}
#
# Data sources
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_vpc" "default" {
  default = true
}
#
data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}


data "aws_secretsmanager_secret" "password" {
  name       = "test-db-password"
  depends_on = [aws_secretsmanager_secret_version.password]
}

data "aws_secretsmanager_secret_version" "password" {
  secret_id = data.aws_secretsmanager_secret.password
}

# Security Groups
resource "aws_security_group" "rds" {
  name_prefix = "${var.app_name}-rds-"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs_tasks.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.app_name}-rds"
  }
}

resource "aws_security_group" "ecs_tasks" {
  name_prefix = "${var.app_name}-ecs-"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = var.app_port
    to_port     = var.app_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.app_name}-ecs-tasks"
  }
}
#
# RDS Subnet Group
resource "aws_db_subnet_group" "main" {
  name       = "${var.app_name}-db-subnet-group"
  subnet_ids = data.aws_subnets.default.ids

  tags = {
    Name = "${var.app_name} DB subnet group"
  }
}

# RDS Parameter Group
resource "aws_db_parameter_group" "main" {
  family = "postgres15"
  name   = "${var.app_name}-db-params"

  parameter {
    name  = "log_statement"
    value = "all"
  }

  tags = {
    Name = "${var.app_name}-db-params"
  }
}
#
# RDS Instance
resource "aws_db_instance" "main" {
  identifier = "${var.app_name}-db"

  engine         = "postgres"
  engine_version = "15.7"
  instance_class = var.db_instance_class

  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = var.db_max_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = var.db_name
  username = var.db_username
  password = data.aws_secretsmanager_secret_version.password

  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name
  parameter_group_name   = aws_db_parameter_group.main.name

  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Sun:04:00-Sun:05:00"

  skip_final_snapshot = true
  deletion_protection = false

  tags = {
    Name = "${var.app_name}-database"
  }
}
#
# # S3 Bucket
resource "aws_s3_bucket" "app_files" {
  bucket_prefix = "${var.app_name}-filerepo"

  tags = {
    App       = "${var.app_name}"
    Component = "filerepo"
  }
}

resource "aws_s3_bucket_versioning" "app_files" {
  bucket = aws_s3_bucket.app_files.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "app_files" {
  bucket = aws_s3_bucket.app_files.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "app_files" {
  bucket = aws_s3_bucket.app_files.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
#
# # ECR Repository
# resource "aws_ecr_repository" "app" {
#   name                 = var.app_name
#   image_tag_mutability = "MUTABLE"
#
#   image_scanning_configuration {
#     scan_on_push = true
#   }
#
#   tags = {
#     Name = "${var.app_name} container repository"
#   }
# }
#
# # ECS Cluster
# resource "aws_ecs_cluster" "main" {
#   name = "${var.app_name}-cluster"
#
#   setting {
#     name  = "containerInsights"
#     value = "enabled"
#   }
#
#   tags = {
#     Name = "${var.app_name} ECS Cluster"
#   }
# }
#
# # CloudWatch Log Group
# resource "aws_cloudwatch_log_group" "app" {
#   name              = "/ecs/${var.app_name}"
#   retention_in_days = 7
#
#   tags = {
#     Name = "${var.app_name} logs"
#   }
# }
#
# # IAM Role for ECS Task Execution
# resource "aws_iam_role" "ecs_task_execution" {
#   name = "${var.app_name}-ecs-task-execution"
#
#   assume_role_policy = jsonencode({
#     Version = "2012-10-17"
#     Statement = [
#       {
#         Action = "sts:AssumeRole"
#         Effect = "Allow"
#         Principal = {
#           Service = "ecs-tasks.amazonaws.com"
#         }
#       }
#     ]
#   })
#
#   tags = {
#     Name = "${var.app_name} ECS Task Execution Role"
#   }
# }
#
# resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
#   role       = aws_iam_role.ecs_task_execution.name
#   policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
# }
#
# # IAM Role for ECS Task
# resource "aws_iam_role" "ecs_task" {
#   name = "${var.app_name}-ecs-task"
#
#   assume_role_policy = jsonencode({
#     Version = "2012-10-17"
#     Statement = [
#       {
#         Action = "sts:AssumeRole"
#         Effect = "Allow"
#         Principal = {
#           Service = "ecs-tasks.amazonaws.com"
#         }
#       }
#     ]
#   })
#
#   tags = {
#     Name = "${var.app_name} ECS Task Role"
#   }
# }
#
# # IAM Policy for S3 Access
# resource "aws_iam_role_policy" "ecs_task_s3" {
#   name = "${var.app_name}-ecs-s3-access"
#   role = aws_iam_role.ecs_task.id
#
#   policy = jsonencode({
#     Version = "2012-10-17"
#     Statement = [
#       {
#         Effect = "Allow"
#         Action = [
#           "s3:GetObject",
#           "s3:PutObject",
#           "s3:DeleteObject",
#           "s3:ListBucket"
#         ]
#         Resource = [
#           aws_s3_bucket.app_files.arn,
#           "${aws_s3_bucket.app_files.arn}/*"
#         ]
#       }
#     ]
#   })
# }
#
# # ECS Task Definition
# resource "aws_ecs_task_definition" "app" {
#   family                   = var.app_name
#   requires_compatibilities = ["FARGATE"]
#   network_mode             = "awsvpc"
#   cpu                      = var.ecs_cpu
#   memory                   = var.ecs_memory
#   execution_role_arn       = aws_iam_role.ecs_task_execution.arn
#   task_role_arn           = aws_iam_role.ecs_task.arn
#
#   container_definitions = jsonencode([
#     {
#       name  = var.app_name
#       image = "${aws_ecr_repository.app.repository_url}:latest"
#
#       portMappings = [
#         {
#           containerPort = var.app_port
#           protocol      = "tcp"
#         }
#       ]
#
#       environment = [
#         {
#           name  = "TAIGA_APP_PORT"
#           value = tostring(var.app_port)
#         },
#         {
#           name  = "TAIGA_APP_ENVIRONMENT"
#           value = "production"
#         },
#         {
#           name  = "TAIGA_DB_DRIVER"
#           value = "postgres"
#         },
#         {
#           name  = "TAIGA_DB_HOST"
#           value = aws_db_instance.main.address
#         },
#         {
#           name  = "TAIGA_DB_PORT"
#           value = "5432"
#         },
#         {
#           name  = "TAIGA_DB_NAME"
#           value = var.db_name
#         },
#         {
#           name  = "TAIGA_DB_USER"
#           value = var.db_username
#         },
#         {
#           name  = "TAIGA_DB_PASSWORD"
#           value = data.aws_secretsmanager_secret_version.password
#         },
#         {
#           name  = "TAIGA_DB_SSL_MODE"
#           value = "require"
#         },
#         {
#           name  = "TAIGA_FILE_REPO_TYPE"
#           value = "s3"
#         },
#         {
#           name  = "TAIGA_FILE_REPO_S3_BUCKET"
#           value = aws_s3_bucket.app_files.bucket
#         },
#         {
#           name  = "TAIGA_FILE_REPO_S3_REGION"
#           value = var.aws_region
#         },
#         {
#           name  = "TAIGA_FILE_REPO_S3_PREFIX"
#           value = "files"
#         },
#         {
#           name  = "TAIGA_FILE_REPO_PREFIX_LENGTH"
#           value = "2"
#         }
#       ]
#
#       logConfiguration = {
#         logDriver = "awslogs"
#         options = {
#           "awslogs-group"         = aws_cloudwatch_log_group.app.name
#           "awslogs-region"        = var.aws_region
#           "awslogs-stream-prefix" = "ecs"
#         }
#       }
#
#       essential = true
#     }
#   ])
#
#   tags = {
#     Name = "${var.app_name} Task Definition"
#   }
# }
#
# # ECS Service
# resource "aws_ecs_service" "app" {
#   name            = var.app_name
#   cluster         = aws_ecs_cluster.main.id
#   task_definition = aws_ecs_task_definition.app.arn
#   desired_count   = 1
#   launch_type     = "FARGATE"
#
#   network_configuration {
#     subnets          = data.aws_subnets.default.ids
#     security_groups  = [aws_security_group.ecs_tasks.id]
#     assign_public_ip = true
#   }
#
#   tags = {
#     Name = "${var.app_name} ECS Service"
#   }
# }
