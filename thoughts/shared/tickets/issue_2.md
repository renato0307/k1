# Summary
The describe is not showing spec information:

Example:

```
Name:         nats-proxy
Namespace:    kube-gloo-system
Kind:         Deployment
API Version:  apps/v1
Labels:       gateway-proxy-id=nats-proxy
              gloo=gateway-proxy
              helm.toolkit.fluxcd.io/name=gloo-ee
              helm.toolkit.fluxcd.io/namespace=kube-gloo-system
              k8slens-edit-resource-version=v1
              app=gloo
              app.kubernetes.io/managed-by=Helm
Created:      2022-06-06 17:59:03 +0100 WEST

Status:
  availableReplicas: 3
  conditions:
  - lastTransitionTime: "2025-10-02T13:39:58Z"
    lastUpdateTime: "2025-10-06T14:22:15Z"
    message: ReplicaSet "nats-proxy-7b9bbb4bc4" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  - lastTransitionTime: "2025-10-07T09:13:34Z"
    lastUpdateTime: "2025-10-07T09:13:34Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
  observedGeneration: 309
  readyReplicas: 3
  replicas: 3
  updatedReplicas: 3

Events:
  <none>
```