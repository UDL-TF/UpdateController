# UpdateController Helm Chart

This Helm chart deploys the UpdateController for managing TF2 game server updates in a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- A PersistentVolume provisioner (if using persistence)

## Installation

### Install from OCI Registry

The chart is published to GitHub Container Registry as an OCI artifact:

```bash
helm install update-controller oci://ghcr.io/udl-tf/charts/update-controller --version 0.1.0
```

### Install from local chart

```bash
# From the repository root
helm install update-controller ./helm

# Or from the helm directory
cd helm
helm install update-controller .
```

### Install with custom values

```bash
helm install update-controller ./helm -f custom-values.yaml
```

### Install with specific parameters

```bash
helm install update-controller ./helm \
  --set image.tag=v1.0.0 \
  --set config.checkInterval=15m \
  --set persistence.size=100Gi
```

## Configuration

The following table lists the configurable parameters of the UpdateController chart and their default values.

| Parameter                      | Description                        | Default                            |
| ------------------------------ | ---------------------------------- | ---------------------------------- |
| `replicaCount`                 | Number of controller replicas      | `1`                                |
| `image.repository`             | Container image repository         | `ghcr.io/udl-tf/update-controller` |
| `image.pullPolicy`             | Image pull policy                  | `Always`                           |
| `image.tag`                    | Image tag (overrides appVersion)   | `""`                               |
| `serviceAccount.create`        | Create service account             | `true`                             |
| `serviceAccount.name`          | Service account name               | `""` (generated)                   |
| `rbac.create`                  | Create RBAC resources              | `true`                             |
| `config.checkInterval`         | Interval to check for updates      | `30m`                              |
| `config.steamAppId`            | Steam app ID                       | `232250`                           |
| `config.gameMountPath`         | Path where game files are mounted  | `/tf`                              |
| `config.podSelector`           | Label selector for pods to restart | `app=tf2-server`                   |
| `config.maxRetries`            | Maximum number of retries          | `3`                                |
| `config.namespace`             | Namespace where game servers run   | `game-servers`                     |
| `resources.limits.cpu`         | CPU limit                          | `500m`                             |
| `resources.limits.memory`      | Memory limit                       | `512Mi`                            |
| `resources.requests.cpu`       | CPU request                        | `100m`                             |
| `resources.requests.memory`    | Memory request                     | `128Mi`                            |
| `persistence.enabled`          | Enable persistent storage          | `true`                             |
| `persistence.existingClaim`    | Use existing PVC                   | `""`                               |
| `persistence.size`             | PVC size                           | `50Gi`                             |
| `persistence.storageClassName` | Storage class name                 | `standard`                         |
| `namespace.create`             | Create namespace                   | `true`                             |
| `namespace.name`               | Namespace name                     | `game-servers`                     |

## Examples

### Using an existing PVC

```yaml
# custom-values.yaml
persistence:
  enabled: true
  existingClaim: my-existing-game-files-pvc
```

```bash
helm install update-controller ./helm -f custom-values.yaml
```

### Deploying to a different namespace

```yaml
# custom-values.yaml
namespace:
  create: false
  name: my-custom-namespace
```

### Custom update interval and retry settings

```yaml
# custom-values.yaml
config:
  checkInterval: '15m'
  maxRetries: '5'
  retryDelay: '10m'
```

### Using a specific image version

```bash
helm install update-controller ./helm --set image.tag=v1.0.0
```

## Upgrading

### Upgrade from OCI Registry

```bash
helm upgrade update-controller oci://ghcr.io/udl-tf/charts/update-controller --version 0.1.0
```

### Upgrade from local chart

```bash
helm upgrade update-controller ./helm
```

With custom values:

```bash
helm upgrade update-controller ./helm -f custom-values.yaml
```

## Uninstallation

```bash
helm uninstall update-controller
```

**Note:** This will not delete the PVC by default. To delete it:

```bash
kubectl delete pvc -n game-servers update-controller-game-files
```

## Troubleshooting

### Check controller status

```bash
kubectl get pods -n game-servers -l app.kubernetes.io/name=update-controller
```

### View controller logs

```bash
kubectl logs -n game-servers -l app.kubernetes.io/name=update-controller -f
```

### Verify RBAC permissions

```bash
kubectl describe clusterrole update-controller
kubectl describe clusterrolebinding update-controller
```

### Check configuration

```bash
kubectl get configmap -n game-servers update-controller-config -o yaml
```

## License

MIT
