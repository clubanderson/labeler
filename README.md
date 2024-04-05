# LABELER
<img src="kube-labeler.jpg" alt="drawing" width="100"/> 

## A labeler for all kubectl, kustomize, and helm resources  

**Challenge**: When working with Kubernetes objects it is necessary to find objects that are part of the same collection. Labels and annotations are a good way to flag objects so that they can be operated on as a collection. For instance, for an nginx package it would be useful to label the deployment, service, service account, and configmap that comes along with it. You could simply just use a namespace as a collection identifier. But a namespace will not help you identify cluster-scoped objects (which includes a namespace where the deployment, service, service account, and configmap reside) as part of a collection.

Common labels are used by Helm to identify objects that are part of the same collection. Most commonly used is:
    
    app.kubernetes.io/part-of: <your collection name here>

You set a label with:
  
  For kubectl
    
    kubectl label <object-type> <object-name> <label-key>=<label-value>

  For Helm
    
    helm install my-release my-chart --set labels.<label-key>=<label-value>

You can then use kubectl to get all items that contain the label you specified
    
    kubectl get all --selector=app.kubernetes.io/part-of=nginx

You would be quick to point out that helm and kubectl all have a means of labeling objects during installation/create/apply. This is partly true, if a) the project offers a well-formed, best-practices helm chart, b) if you do not use 'kubectl apply -f', and c) if you do not use 'kubectl -k' (kustomize) to install the application

Yes, for kubectl you can add labels to your source before applying with 'kubectl apply -f'. Yes, you can do the same for your helm source. And, yes, you can add labels to your kustomize object source files before applying with 'kubectl -k'. This is time-consuming work and requires you clone the source and manipulate it locally and possibly source control it for others to use. Imagine having to do this many times for different project source files. I am a platform engineer, and I can tell you that this is a tedious process. All it takes is a single upstream change/update and you need to read, edit, and store your version of the source files again.

After hacking at this for some time, I decided to come up with a command that works kinda like grep. You can run grep against a file as input or run grep against a command as output (linux pipe command)

    grep "apple" example.txt

  or

    echo "This is an example text with an apple" | grep "apple"

Why not create a command that can do the same for labeling Kubernetes resources

    labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind /path/to/myapp

  or

    (helm install with --debug mode)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

  or

    (helm install without --debug mode)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace |  ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

  or

    (helm template run - no installation)   
    helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run |  ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

  or

    (helm --dry-run - no installation)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

  or
    
    (kubectl apply -f)
    kubectl apply -f deployment.yml | ./labeler -l app.kubernetes.io/part-of=my-kubectl-app -k ~/.kube/config -c kind-kind

  or
    
    (kustomize - 'kubectl -k')
    kubectl -k kustomization.yml | ./labeler -l app.kubernetes.io/part-of=my-kustomize-app -k ~/.kube/config -c kind-kind


The result, in all cases, would be output of the yaml used to create resources and then labeling with your desired label. If you are running in template or --dry-run where there is no 'apply' of the object definitions, then the label commands are furnished as output





### UNDER CONSTRUCTION:



new to try:
    KUBECONFIG=~/.kube/config helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
    KUBECONFIG=~/.kube/config helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug


  install with debug (native yaml - resources applied, labeling succeeds)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

  install with dry-run (native yaml - but does not apply resources, so labeling may not work unless resource exist from a previous helm install - which is cool)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

  template run (native yaml - but does not apply resources, so labeling may not work unless resource exist from a previous helm install - which is cool)
    helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

  plain install (uses history hack)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets


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