# Terraform AWS Infrastructure

This directory contains Terraform configuration for deploying the Go application to AWS.

## Infrastructure Overview

- **RDS PostgreSQL**: Single-AZ database instance for data storage
- **S3**: Bucket for file storage (content-addressable files)
- **ECS Fargate**: Single task container deployment
- **ECR**: Container registry for Docker images
- **IAM**: Roles and policies for secure access
- **CloudWatch**: Logging and monitoring

## Prerequisites

1. **AWS CLI** configured with appropriate credentials
2. **Terraform** >= 1.0 installed
3. **Docker** for building container images

## Deployment Steps

### 1. Configure Variables
```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
```

### 2. Initialize Terraform
```bash
terraform init
```

### 3. Plan Infrastructure
```bash
terraform plan
```

### 4. Deploy Infrastructure
```bash
terraform apply
```

### 5. Build and Push Container
```bash
# Get ECR login token
aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <ecr-url>

# Build and tag image
docker build -t taiga .
docker tag taiga:latest <ecr-url>:latest

# Push to ECR
docker push <ecr-url>:latest
```

### 6. Update ECS Service
```bash
# Force new deployment to pull latest image
aws ecs update-service --cluster taiga-cluster --service taiga --force-new-deployment
```

## Configuration

### Required Variables
- `db_password`: Secure database password

### Optional Variables
- `aws_region`: AWS region (default: us-east-2)
- `app_name`: Application name (default: taiga)
- `db_instance_class`: RDS instance size (default: db.t3.micro)
- `ecs_cpu`/`ecs_memory`: Container resources

## Outputs

After deployment, Terraform provides:
- Database connection details
- S3 bucket name
- ECR repository URL
- ECS cluster/service names
- Complete app configuration

## Application Access

The ECS task will have a public IP address. To find it:
```bash
# Get task ARN
aws ecs list-tasks --cluster taiga-cluster --service-name taiga

# Get task details including public IP
aws ecs describe-tasks --cluster taiga-cluster --tasks <task-arn>
```

## Cleanup

```bash
terraform destroy
```

## Cost Optimization

- Uses `db.t3.micro` RDS instance (free tier eligible)
- ECS Fargate with minimal CPU/memory
- S3 with lifecycle policies
- Single-AZ deployment for cost savings

## Security Notes

- Database in default VPC private subnets
- Security groups restrict access appropriately
- S3 bucket with public access blocked
- IAM roles follow least-privilege principle
- Database password should be managed securely
