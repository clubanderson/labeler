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
        helm install my-release-name repo-name/chart-name ./labeler

to build:

    go build labeler.go
    chmod +x labeler.go

to test:

    helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
    helm install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets | ./labeler

    - another -

    helm install my-release stable/nginx-ingress | ./labeler
