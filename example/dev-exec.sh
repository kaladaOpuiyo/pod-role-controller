#!/bin/bash 

NAMESPACES='ops'
ROLE_NAME='test-role'

for namespace in $NAMESPACES
do
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: $ROLE_NAME
  namespace: $namespace
rules:
- apiGroups:
  - ""
  resources:
  - pods/portforward
  - pods/exec
  - pods/logs 
  verbs:
  - list
  - create
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: $ROLE_NAME
  namespace: $namespace 
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: $ROLE_NAME 
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: ops 
EOF
echo "$ROLE_NAME role and rolebinding created in $namespace"

done
