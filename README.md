# Kubernetes Labeler
<img src="kube-labeler.jpg" alt="drawing" width="100"/> 

## A labeler for all kubectl, kustomize, and helm resources  

<!-- [![Watch the video](https://youtu.be/lHcn3W7iuqA)](https://youtu.be/lHcn3W7iuqA) -->
<img src="demo/labeler.gif" width="800">


[Video Demo](https://youtu.be/lHcn3W7iuqA)


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

After hacking at this for some time, I decided to come up with 2 approaches to resolve this:

(check out tests/test.sh for a full battery of tests)

# Install with brew, or clone and build
```
    brew tap clubanderson/labeler https://github.com/clubanderson/labeler
    brew install labeler
``` 
  - or -
```
    git clone https://github.com/clubanderson/labeler
    cd labeler
    make build
    make deploy
```
  - then -
```
    alias k="labeler kubectl"
    alias h="labeler helm"
```
  - optional - edit your rc file (./zshrc) (or just run these commands on your local shell)
```
    alias k='labeler kubectl'  # you could also replace 'kubectl' (looking into this)
    alias h='labeler helm'     # you could also replace 'helm' (looking into this)
```

# 1 - Labeler as an alias to kubectl and helm

run k as you would an kubectl command with arguments, and labeler will label all applied/created resources, or give output on how to do so:

  kubectl

    k apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite
    
    deployment.apps/my-app-deployment2 unchanged
    service/my-app-service2 unchanged

  kustomize

  kustomize with "" or "default" namespace (object were previously created and labeled)

    k apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default --overwrite
      service/my-app-service already has label app.kubernetes.io/part-of=sample-app
      deployment.apps/my-app-deployment already has label app.kubernetes.io/part-of=sample-app

  kustomize with "" or "default" namespace (object were previously created and but new label value provided)

    k apply -k examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite 
      🏷️ created and labeled object "my-app-service" in namespace "default" with app.kubernetes.io/part-of=sample
      🏷️ created and labeled object "my-app-deployment" in namespace "default" with app.kubernetes.io/part-of=sample

  kustomize with a namespace other than "" or "default" (objects were previously created and labeled)

    k apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=temp --overwrite
      service/my-app-service already has label app.kubernetes.io/part-of=sample-app
      deployment.apps/my-app-deployment already has label app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/namespaces "temp" with app.kubernetes.io/part-of=sample-app

  kustomize with a namespace other than "" or "default" (objects were previously created but new label value provided)

    k apply -k examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite
      🏷️ created and labeled object "my-app-service" in namespace "temp" with app.kubernetes.io/part-of=sample
      🏷️ created and labeled object "my-app-deployment" in namespace "temp" with app.kubernetes.io/part-of=sample
      🏷️ labeled object /v1/namespaces "temp" with app.kubernetes.io/part-of=sample

  run h with any helm command line arguments, and labeler will label all installed resources, or give output on how to do so

  helm (template)

    h --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

      🏷️ labeled object /v1/namespaces "sealed-secrets" with app.kubernetes.io/part-of=sample-app

      The following resources do not exist and can be labeled at a later time:

      kubectl label serviceaccounts sealed-secrets app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label clusterroles secrets-unsealer app.kubernetes.io/part-of=sample-app
      kubectl label clusterrolebindings sealed-secrets app.kubernetes.io/part-of=sample-app
      kubectl label roles sealed-secrets-key-admin app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label roles sealed-secrets-service-proxier app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label rolebindings sealed-secrets-key-admin app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label rolebindings sealed-secrets-service-proxier app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label services sealed-secrets app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label services sealed-secrets-metrics app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label deployments sealed-secrets app.kubernetes.io/part-of=sample-app -n sealed-secrets

  helm (install with dry-run)

    h --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

    The following resources do not exist and can be labeled at a later time:

      kubectl label serviceaccounts sealed-secrets app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label clusterroles secrets-unsealer app.kubernetes.io/part-of=sample-app
      kubectl label clusterrolebindings sealed-secrets app.kubernetes.io/part-of=sample-app
      kubectl label roles sealed-secrets-key-admin app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label roles sealed-secrets-service-proxier app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label rolebindings sealed-secrets-key-admin app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label rolebindings sealed-secrets-service-proxier app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label services sealed-secrets app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label services sealed-secrets-metrics app.kubernetes.io/part-of=sample-app -n sealed-secrets
      kubectl label deployments sealed-secrets app.kubernetes.io/part-of=sample-app -n sealed-secrets

  helm (install)

    h --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

      🏷️ labeled object /v1/serviceaccounts "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/clusterroles "secrets-unsealer" in namespace "" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/clusterrolebindings "sealed-secrets" in namespace "" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/services "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/services "sealed-secrets-metrics" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object apps/v1/deployments "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/namespaces "sealed-secrets" with app.kubernetes.io/part-of=sample-app

# Labeler with a sample OCM ManifestWork as output
Not entirely working, but a sample manifestwork is output (TODO: output only right now)

  with kubectl and kustomize:

    k apply -f examples/kubectl/pass --label=app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite --l-mw-create

    deployment.apps/my-app-deployment2 unchanged
    service/my-app-service2 unchanged
      deployment.apps/my-app-deployment2 already has label app.kubernetes.io/part-of=sample
      service/my-app-service2 already has label app.kubernetes.io/part-of=sample
      🏷️ labeled object /v1/namespaces "temp" with app.kubernetes.io/part-of=sample

    apiVersion: work.open-cluster-management.io/v1
    kind: ManifestWork
    metadata:
        name: change-me
    spec:
        workload:
            manifests: []

# Labeler with a KubeStellar BindingPolicy as output
You can give command line arguments to trigger the output (and creation) of a bindingpolicy for use with KubeStellar

    --l-bp-create 
    --l-bp-clusterselector=location-group=edge 
    --l-bp-wantsingletonreportedstate 
    --l-bp-name=my-test
    --l-bp-wds=wds1 (this triggers an attempt to create the bindingpolicy object)


  with kubectl and kustomize:

    k --context=kind-kind apply -f examples/kubectl/pass --label=app.kubernetes.io/part-of=sample2 -n temp --overwrite --l-bp-create --l-bp-clusterselector=location-group=edge --l-bp-wantsingletonreportedstate --l-bp-name=my-test --l-bp-wds=wds1

    deployment.apps/my-app-deployment2 unchanged
    service/my-app-service2 unchanged
      deployment.apps/my-app-deployment2 already has label app.kubernetes.io/part-of=sample2
      service/my-app-service2 already has label app.kubernetes.io/part-of=sample2
      🏷️ labeled object /v1/namespaces "temp" with app.kubernetes.io/part-of=sample2

      🚀 Attempting to create BindingPolicy object "my-test" in WDS namespace "wds1"


  with helm:

    h --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --l-bp-create --l-bp-clusterselector=location-group=edge --l-bp-wantsingletonreportedstate --l-bp-name=my-test; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

    NAME: sealed-secrets
    LAST DEPLOYED: Tue Apr  9 12:01:43 2024
    NAMESPACE: sealed-secrets
    STATUS: deployed
    REVISION: 1
    TEST SUITE: None
    NOTES:
    ** Please be patient while the chart is being deployed **

    You should now be able to create sealed secrets.

    1. Install the client-side tool (kubeseal) as explained in the docs below:

        https://github.com/bitnami-labs/sealed-secrets#installation-from-source

    2. Create a sealed secret file running the command below:

        kubectl create secret generic secret-name --dry-run=client --from-literal=foo=bar -o [json|yaml] | \
        kubeseal \
          --controller-name=sealed-secrets \
          --controller-namespace=sealed-secrets \
          --format yaml > mysealedsecret.[json|yaml]

    The file mysealedsecret.[json|yaml] is a commitable file.

    If you would rather not need access to the cluster to generate the sealed secret you can run:

        kubeseal \
          --controller-name=sealed-secrets \
          --controller-namespace=sealed-secrets \
          --fetch-cert > mycert.pem

    to retrieve the public cert used for encryption and store it locally. You can then run 'kubeseal --cert mycert.pem' instead to use the local cert e.g.

        kubectl create secret generic secret-name --dry-run=client --from-literal=foo=bar -o [json|yaml] | \
        kubeseal \
          --controller-name=sealed-secrets \
          --controller-namespace=sealed-secrets \
          --format [json|yaml] --cert mycert.pem > mysealedsecret.[json|yaml]

    3. Apply the sealed secret

        kubectl create -f mysealedsecret.[json|yaml]

    Running 'kubectl get secret secret-name -o [json|yaml]' will show the decrypted secret that was generated from the sealed secret.

    Both the SealedSecret and generated Secret must have the same name and namespace.
      🏷️ labeled object /v1/serviceaccounts "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/clusterroles "secrets-unsealer" in namespace "" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/clusterrolebindings "sealed-secrets" in namespace "" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/services "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/services "sealed-secrets-metrics" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object apps/v1/deployments "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/namespaces "sealed-secrets" with app.kubernetes.io/part-of=sample-app

    apiVersion: control.kubestellar.io/v1alpha1
    kind: BindingPolicy
    metadata:
        name: my-test
    wantSingletonReportedState: true
    clusterSelectors:
        - matchLabels:
            location-group: edge
    downsync:
        - objectSelectors:
            - matchLabels:
                app.kubernetes.io/part-of: sample-app

# Labeler with deployment to multiple contexts
If your in a jam and need kubectl or helm to deploy to multiple context, just add "--l-remote-contexts=wds1,wds2" to quickly deploy packages to multiple remote contexts. Assumes all contexts are in the original kubeconfig. (TODO - add params list of --kubeconfig in --l-remote-kubeconfigs and iterate to find the right one, then break)

  with kubectl:

    k apply -f examples/kubectl/pass --label=app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite --l-remote-contexts=wds1,wds2  kind-kind/default ⎈ 

    deployment.apps/my-app-deployment2 unchanged
    service/my-app-service2 unchanged
      deployment.apps/my-app-deployment2 already has label app.kubernetes.io/part-of=sample
      service/my-app-service2 already has label app.kubernetes.io/part-of=sample
      🏷️ labeled object /v1/namespaces "temp" with app.kubernetes.io/part-of=sample

    attempting deployment to contexts: [wds1 wds2]
    error: context "wds1" does not exist
    exit status 1
    error: context "wds2" does not exist
    exit status 1

  with helm:

    h --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --l-remote-contexts=wds1,wds2; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets 
    NAME: sealed-secrets
    LAST DEPLOYED: Tue Apr  9 13:29:24 2024
    NAMESPACE: sealed-secrets
    ...
    Both the SealedSecret and generated Secret must have the same name and namespace.
      🏷️ labeled object /v1/serviceaccounts "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object rbac.authorization.k8s.io/v1/clusterroles "secrets-unsealer" in namespace "" with app.kubernetes.io/part-of=sample-app
      🏷️ labeled object /v1/namespaces "sealed-secrets" with app.kubernetes.io/part-of=sample-app
    ...

    attempting deployment to contexts: [wds1 wds2]
    Error: INSTALLATION FAILED: Kubernetes cluster unreachable: context "wds1" does not exist
    exit status 1
    Error: INSTALLATION FAILED: Kubernetes cluster unreachable: context "wds2" does not exist
    exit status 1

# 2 - a command that works kinda like grep. You can run grep against a file as input or run grep against a command as output (linux pipe command)

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
    kubectl apply -k kustomization.yml | ./labeler -l app.kubernetes.io/part-of=my-kustomize-app -k ~/.kube/config -c kind-kind


The result, in all cases, would be output of the yaml used to create resources and then labeling with your desired label. If you are running in template or --dry-run where there is no 'apply' of the object definitions, then the label commands are furnished as output



Traditional use of helm

    helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug

Using labeler as a piped command

  install with debug (native yaml - resources applied, labeling succeeds)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

  install with dry-run (native yaml - but does not apply resources, so labeling may not work unless resource exist from a previous helm install - which is cool)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

  template run (native yaml - but does not apply resources, so labeling may not work unless resource exist from a previous helm install - which is cool)
    helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

  plain install (uses history hack)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets


works like this...

    kubectl (bunch of files in a path)
        (without error from kubectl)
          kubectl --context=kind-kind apply -f ./examples/kubectl/pass | ./labeler -l app.kubernetes.io/part-of=sample-value
        (with error returning from kubectl)
          kubectl --context=kind-kind apply -f ./examples/kubectl/fail | ./labeler -l app.kubernetes.io/part-of=sample-value 
    
    kubectl (single file)
        kubectl --context=kind-kind apply -f ./examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=another-sample-value
    
    kustomize
        kubectl --context=kind-kind apply -k ./examples/kustomize | ./labeler -l app.kubernetes.io/part-of=sample-value

    helm (local chart)
        helm --kube-context=kind-kind install my-release-name ./mychart | ./labeler app.kubernetes.io/part-of=my-release-value

    helm (remote chart)
        helm --kube-context=kind-kind repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
        helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler app.kubernetes.io/part-of=sample-value

## get started:

You need a kubernetes, go, kubectl, helm environment  - create one with Kind:
[Zero to Kube and GO in 90 Seconds](https://clubanderson.medium.com/zero-to-kube-and-go-in-90-seconds-f6f4730ab265)

### Install with brew, or clone and build
```
    brew tap clubanderson/labeler https://github.com/clubanderson/labeler
    brew install labeler
``` 
  or -
```
    git clone https://github.com/clubanderson/labeler
    cd labeler
    make build
    make deploy
```
  then -
```
    alias k="labeler kubectl"
    alias h="labeler helm"
```
  optional - edit your rc file (./zshrc) (or just run these commands on your local shell)
```
    alias k='labeler kubectl'  # you could also replace 'kubectl' (looking into this)
    alias h='labeler helm'     # you could also replace 'helm' (looking into this)
```
  
### to test:

    helm --kube-context=kind-kind repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
    
    passing test:
      helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value
    
    failing test ('-l' missing from command)
      helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value
    
    - or -

    passing test:
      helm --kube-context=kind-kind install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --create-namespace --version 1.2.0 | ./labeler -l app.kubernetes.io/part-of=sample-value

    - or -

    failing test (missing kubeconfig)
      helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig eks.config --context kind-kind


    passing test (context and kubeconfig exist)
      helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig ~/.kube/config --context kind-kind

    - or - (on ubuntu)

    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

    (note the use of 'exec' to get the command into history)
    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace > exec | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

### to reset:

    helm uninstall sealed-secrets -n sealed-secrets
    helm uninstall nginx -n nginx

### sample output:

    #1

    helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig ~/.kube/config --context kind-kind
    
    data is from pipe
    YAML data detected in stdin
        🏷️ labeled object rbac.authorization.k8s.io/v1/clusterroles "secrets-unsealer" in namespace "" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/clusterrolebindings "sealed-secrets" in namespace "" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object /v1/services "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object /v1/services "sealed-secrets-metrics" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
    helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler


    #2

    helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler -l app.kubernetes.io/part-of=sample-value

    data is from pipe
    Pulled: ghcr.io/nginxinc/charts/nginx-ingress:1.2.0
    Digest: sha256:6656e80c7975c393ea36bdfea3987f87d119c7d1501ba01eea89b739b69381bd
    Error: INSTALLATION FAILED: cannot re-use a name that is still in use
    No YAML data detected in stdin, will try to run again with YAML output
    mac
    command found: "helm"
    original command: "helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run"

    running command: helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run 
        🏷️ labeled object /v1/serviceaccounts "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/clusterroles "secrets-unsealer" in namespace "" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/clusterrolebindings "sealed-secrets" in namespace "" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/roles "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-key-admin" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object rbac.authorization.k8s.io/v1/rolebindings "sealed-secrets-service-proxier" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object /v1/services "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object /v1/services "sealed-secrets-metrics" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
        🏷️ labeled object apps/v1/deployments "sealed-secrets" in namespace "sealed-secrets" with app.kubernetes.io/part-of=sample-value
