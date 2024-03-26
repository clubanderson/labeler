# hackathon-labeler

# LABELER
A labeler for all kubectl, kustomize, and helm resources...  how does it work
HACKME!!!


should work like this...

    kubectl (bunch of files in a path)
        kubectl apply -f some/path/with/yaml/files | ./labeler

    kubectl (single file)
        kubectl apply -f a.yaml-file.yml | ./labeler
    
    kustomize
        kubectl apply -k some/path/with/kustomization.yml | ./labeler

    helm (local chart)
        helm install my-release-name ./mychart | ./labeler

    helm (remote chart)
        helm repo add chart-name repo-name
        helm install my-release-name repo-name/chart-name --version 1.0.1 --create-namespace ./labeler

to build:

    go build labeler.go
    chmod +x labeler.go

to test:

    helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
    helm install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets | ./labeler

    - or -

    helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler
 
to reset:

    helm uninstall sealed-secrets -n sealed-secrets
    helm uninstall nginx -n nginx


sample output:

    helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler


    Original command: "helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0"
    your running helm
    [template nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0]
    Pulled: ghcr.io/nginxinc/charts/nginx-ingress:1.2.0
    Digest: sha256:6656e80c7975c393ea36bdfea3987f87d119c7d1501ba01eea89b739b69381bd
    APIVERSION: V1
    KIND: SERVICEACCOUNT
      NAME: NGINX-NGINX-INGRESS
    APIVERSION: V1
    KIND: CONFIGMAP
      NAME: NGINX-NGINX-INGRESS
    APIVERSION: V1
    KIND: CONFIGMAP
      NAME: NGINX-INGRESS-LEADER
    KIND: CLUSTERROLE
    APIVERSION: RBAC.AUTHORIZATION.K8S.IO/V1
      NAME: NGINX-NGINX-INGRESS
    KIND: CLUSTERROLEBINDING
    APIVERSION: RBAC.AUTHORIZATION.K8S.IO/V1
      NAME: NGINX-NGINX-INGRESS
      NAME: NGINX-NGINX-INGRESS
      NAME: NGINX-NGINX-INGRESS
    KIND: ROLE
    APIVERSION: RBAC.AUTHORIZATION.K8S.IO/V1
      NAME: NGINX-NGINX-INGRESS
    KIND: ROLEBINDING
    APIVERSION: RBAC.AUTHORIZATION.K8S.IO/V1
      NAME: NGINX-NGINX-INGRESS
      NAME: NGINX-NGINX-INGRESS
      NAME: NGINX-NGINX-INGRESS
    APIVERSION: V1
    KIND: SERVICE
      NAME: NGINX-NGINX-INGRESS-CONTROLLER
    APIVERSION: APPS/V1
    KIND: DEPLOYMENT
      NAME: NGINX-NGINX-INGRESS-CONTROLLER
    APIVERSION: NETWORKING.K8S.IO/V1
    KIND: INGRESSCLASS
      NAME: NGINX