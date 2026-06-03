# AWS EKS Deployment Guide — Sentinel AI

This guide covers deploying Sentinel AI to a production AWS EKS cluster.

---

## Prerequisites

- **AWS CLI v2** configured with admin/deployment IAM credentials
- **eksctl** (`>= 0.170.0`) — [Install guide](https://eksctl.io/installation/)
- **kubectl** (`>= 1.30`) — [Install guide](https://kubernetes.io/docs/tasks/tools/)
- **Docker** running locally
- **Helm** (`>= 3.x`) — for ALB Controller installation

---

## 1. Create the EKS Cluster

```bash
eksctl create cluster -f aws/eksctl-cluster.yml
```

This creates:
- A managed EKS cluster `sentinel-cluster` in `us-east-1`
- 2× `t3.medium` nodes (auto-scales to 4)
- EBS CSI driver for persistent volumes
- IAM policies for autoscaler, CloudWatch, and ALB

Verify:
```bash
kubectl get nodes
kubectl cluster-info
```

---

## 2. Install AWS Load Balancer Controller

The ALB Controller maps Kubernetes Ingress resources to AWS Application Load Balancers.

```bash
# Add the EKS Helm chart repo
helm repo add eks https://aws.github.io/eks-charts
helm repo update

# Install the controller
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=sentinel-cluster \
  --set serviceAccount.create=true \
  --set serviceAccount.name=aws-load-balancer-controller

# Verify
kubectl get deployment -n kube-system aws-load-balancer-controller
```

---

## 3. Build and Push Images to ECR

```bash
chmod +x aws/push-to-ecr.sh
./aws/push-to-ecr.sh
```

This script:
1. Authenticates Docker with ECR
2. Creates ECR repositories (if they don't exist)
3. Builds all three service images
4. Tags and pushes to ECR

---

## 4. Update Image URIs in K8s Manifests

After pushing to ECR, update the `image:` fields in:
- `k8s/05-ingestion-service.yml`
- `k8s/06-processing-service.yml`
- `k8s/07-dashboard.yml`

Replace:
```yaml
image: sentinel/ingestion-service:latest
```
With:
```yaml
image: <ACCOUNT_ID>.dkr.ecr.us-east-1.amazonaws.com/sentinel/ingestion-service:latest
```

Or use Kustomize image overrides:
```bash
kubectl apply -k k8s/ \
  --set-image ingestion-service=<ECR_URI> \
  --set-image processing-service=<ECR_URI> \
  --set-image dashboard=<ECR_URI>
```

---

## 5. Deploy to EKS

```bash
# Apply all manifests via Kustomize
kubectl apply -k k8s/

# Watch rollout
kubectl -n sentinel get pods -w

# Verify all deployments are ready
kubectl -n sentinel get deployments
kubectl -n sentinel get statefulsets
kubectl -n sentinel get services
```

---

## 6. Get the ALB URL

After deploying the Ingress resource (with ALB annotations uncommented in `k8s/10-ingress.yml`):

```bash
kubectl -n sentinel get ingress sentinel-ingress
```

The `ADDRESS` column will show the ALB DNS name (may take 2-3 minutes to provision).

---

## 7. Access Services

| Service | URL |
|---------|-----|
| Dashboard | `http://<ALB_DNS>/` |
| Ingestion API | `http://<ALB_DNS>/api` |
| Grafana | `http://<ALB_DNS>/grafana` (admin/admin) |
| Prometheus | `http://<ALB_DNS>/prometheus` |

---

## Production Recommendations

### Replace Self-Hosted Infrastructure

| Self-Hosted | AWS Managed | Benefit |
|-------------|------------|---------|
| Kafka (CP) | **Amazon MSK** | Managed brokers, auto-patching, multi-AZ |
| Elasticsearch | **Amazon OpenSearch** | Managed clusters, automated snapshots, UltraWarm |
| Ollama (Llama 3.2) | **Amazon Bedrock** | Serverless LLM, no GPU management, pay-per-token |
| ZooKeeper | *(eliminated by MSK)* | MSK handles coordination |

### Security Hardening

- Enable TLS on Ingress (ACM certificate)
- Replace Grafana default password with Secrets Manager
- Enable ES/OpenSearch xpack security
- Use IRSA (IAM Roles for Service Accounts) for fine-grained pod permissions
- Network policies to restrict inter-service traffic
- Enable EKS audit logging to CloudWatch

### Observability

- Enable Container Insights for node/pod metrics
- Set up CloudWatch alarms for critical Prometheus metrics
- Configure PagerDuty/Slack alerts via Alertmanager

### Storage

- Replace `emptyDir` volumes with EBS-backed PVCs
- Configure automated EBS snapshots
- Use `gp3` storage class for better price/performance

---

## Cleanup

```bash
# Delete all Sentinel resources
kubectl delete -k k8s/

# Delete the EKS cluster
eksctl delete cluster --name sentinel-cluster --region us-east-1
```
