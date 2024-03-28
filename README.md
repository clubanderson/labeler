# hackathon-labeler

# LABELER
A labeler for all kubectl, kustomize, and helm resources...  how does it work
HACKME!!!

result - all resources installed via kubectl apply, kubectl -k, and helm install are labeled


should work like this...

    kubectl (bunch of files in a path)
        kubectl --context=kind-kind -f some/path/with/yaml/files | ./labeler app.kubernetes.io/part-of=sample-value

    kubectl (single file)
        kubectl --context=kind-kind apply -f a.yaml-file.yml | ./labeler app.kubernetes.io/part-of=another-sample-value
    
    kustomize
        kubectl --context=kind-kind -k some/path/with/kustomization.yml | ./labeler app.kubernetes.io/part-of=sample-value

    helm (local chart)
        helm --kube-context=kind-kind install my-release-name ./mychart | ./labeler app.kubernetes.io/part-of=my-release-value

    helm (remote chart)
        helm --kube-context=kind-kind repo add chart-name repo-name
        helm --kube-context=kind-kind install my-release-name repo-name/chart-name --version 1.0.1 --create-namespace ./labeler  app.kubernetes.io/part-of=and-another-sample-value

## get started:

You need a kubernetes cluster - create one with Kind
[Zero to Kube and GO in 90 Seconds](https://clubanderson.medium.com/zero-to-kube-and-go-in-90-seconds-f6f4730ab265)

### to build:

    go build labeler.go
    chmod +x labeler

### to test:

    helm --kube-context=kind-kind repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler app.kubernetes.io/part-of=sample-value
    
    - or -

    helm --kube-context=kind-kind install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --create-namespace --version 1.2.0 | ./labeler app.kubernetes.io/part-of=sample-value

    - or -

    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig eks.config --context kind-kind

    - or - (on ubuntu)


    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

### to reset:

    helm uninstall sealed-secrets -n sealed-secrets
    helm uninstall nginx -n nginx


### sample output:

    helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler


    Original command: "helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0"
    your running helm
    [template nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0]
    Pulled: ghcr.io/nginxinc/charts/nginx-ingress:1.2.0
    Digest: sha256:6656e80c7975c393ea36bdfea3987f87d119c7d1501ba01eea89b739b69381bd
    apiVersion: v1
    kind: ServiceAccount
      name: sealed-secrets
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
      name: secrets-unsealer
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
      name: sealed-secrets
      name: secrets-unsealer
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
      name: sealed-secrets-key-admin
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
      name: sealed-secrets-service-proxier
    apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
      name: sealed-secrets-key-admin
      name: sealed-secrets-key-admin
    apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
      name: sealed-secrets-service-proxier
      name: sealed-secrets-service-proxier
    apiVersion: v1
    kind: Service
      name: sealed-secrets
    apiVersion: v1
    kind: Service
      name: sealed-secrets-metrics
    apiVersion: apps/v1
    kind: Deployment
      name: sealed-secrets
    2024/03/28 11:28:52 labeling all resources with: "app.kubernetes.io/part-of=sample-value"