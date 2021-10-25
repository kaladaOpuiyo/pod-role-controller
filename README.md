# POD-ROLE-CONTROLLER 

This controller is used to dynamically associate pods of a deployment to a namespaced role.  
For each of the provided rule's resources, we update the resource names list with the names of pod members of the controller defined deployments.This controller attempts to overcome limitations in the use of label selectors with rbac https://github.com/kubernetes/kubernetes/issues/44703#issuecomment-324826356.

## Deployment

```
helm upgrade pod-role-controller ./chart/pod-role-controller --install --debug --wait --namespace="$NAMESPACE" 
```

## Example 
```
apiVersion: controller.pod.role.controller.io/v1alpha1
kind: PodRoleController
metadata:
  name: pod-role-controller
  namespace: ops
spec:
  deployments:
    - nginx
    - hello-world
  role: test-role
  labelSelector: "app"
  resources:
    - pods/portforward
    - pods/exec
    - pods/logs
```

## Build Image  

```
export IMG= $YOUR_REPO/pod-role-controller:$TAG
make docker-build IMG=$IMG
```

## Build Push  

```
export IMG= $YOUR_REPO/pod-role-controller:$TAG
make docker-push IMG=$IMG
```
