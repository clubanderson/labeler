helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo "---------------------------------------------"
echo "--- (helm install with --debug mode) ---"
echo "---------------------------------------------"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- (helm install without --debug mode) --- should label"
echo "---------------------------------------------"
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace |  ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (helm template run - no installation - installed previously) --- should label"  
helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run |  ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (helm template run - no installation - and uninstalled before running) --- should not label"  
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run |  ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

# echo "---------------------------------------------"
# echo "--- (helm --dry-run - no installation) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

# echo "---------------------------------------------"
# echo "--- (kubectl apply -f) ---"
# kubectl apply -f ../examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=my-kubectl-app -k ~/.kube/config -c kind-kind

# echo "---------------------------------------------"
# echo "--- (kustomize - 'kubectl -k') ---"
# kubectl apply -k ../examples/kustomize | ./labeler -l app.kubernetes.io/part-of=my-kustomize-app -k ~/.kube/config -c kind-kind

# echo "---------------------------------------------"
# echo "--- install with debug (native yaml - resources applied, labeling succeeds) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo "---------------------------------------------"
# echo "--- install with dry-run (native yaml - but does not apply resources, so labeling may not work unless resource exist from a previous helm install - which is cool) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo "---------------------------------------------"
# echo "--- template run (native yaml - but does not apply resources, so labeling may not work unless resource exist from a previous helm install - which is cool) ---"
# helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo "---------------------------------------------"
# echo "--- plain install (uses history hack) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo
# echo "--- kubectl (bunch of files in a path) ---"
# echo "--- (without error from kubectl) ---"
# kubectl --context=kind-kind apply -f ../examples/kubectl/pass | ./labeler -l app.kubernetes.io/part-of=sample-value

# echo "---------------------------------------------"
# echo "--- (with error returning from kubectl) ---"
# kubectl --context=kind-kind apply -f ../examples/kubectl/fail | ./labeler -l app.kubernetes.io/part-of=sample-value 
    
# echo "---------------------------------------------"
# echo "--- kubectl (single file) ---"
# kubectl --context=kind-kind apply -f ../examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=another-sample-value
    
# echo "---------------------------------------------"
# echo "--- kustomize ---"
# kubectl --context=kind-kind apply -k ../examples/kustomize | ./labeler -l app.kubernetes.io/part-of=sample-value

# echo "---------------------------------------------"
# echo "--- helm (local chart) ---"
# helm --kube-context=kind-kind install my-release-name ./mychart | ./labeler app.kubernetes.io/part-of=my-release-value

# echo "---------------------------------------------"
# echo "--- helm (remote chart) ---"
# helm --kube-context=kind-kind repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler app.kubernetes.io/part-of=sample-value

# helm --kube-context=kind-kind repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets

# echo "--- passing test: ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value
    
# echo "--- failing test ('-l' missing from command) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sample-value
    
# echo "--- passing test: ---"
# helm --kube-context=kind-kind install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --create-namespace --version 1.2.0 | ./labeler -l app.kubernetes.io/part-of=sample-value

# echo "--- failing test (missing kubeconfig) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig eks.config --context kind-kind

# echo "--- passing test (context and kubeconfig exist) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig ~/.kube/config --context kind-kind

# echo "--- - or - (on ubuntu) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

# echo "--- (note the use of 'exec' to get the command into history) ---"
# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace > exec | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig ~/.kube/config --context kind-kind
# helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler -l app.kubernetes.io/part-of=sample-value


# echo
# echo "---------------------------------------------"
# echo "               ALIAS TESTS"
# echo "---------------------------------------------"
# alias kl='labeler kubectl'
# alias hl='labeler helm'

# echo
# echo "---------------------------------------------"
# echo "--- kubectl alias with 'sample' label - without --overwrite ---"
# kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default

# echo
# echo "---------------------------------------------"
# echo "--- kubectl alias with 'sample' label - with --overwrite ---"
# kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite

# echo
# echo "---------------------------------------------"
# echo "--- kubectl alias with 'sample' label - with --overwrite ---"
# # 
# kl create namespace temp
# kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite

# echo
# echo "---------------------------------------------"
# echo "--- kubectl alias with 'sample' label - without --overwrite ---"
# kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp

# echo
# echo "---------------------------------------------"
# echo "--- kustomize alias with 'sample-app' label without --overwrite ---"
# kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default

# echo
# echo "---------------------------------------------"
# echo "--- kustomize alias with 'sample-app' label without --overwrite ---"
# kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default


# echo
# echo "---------------------------------------------"
# echo "--- kustomize alias with 'sample' label with --overwrite ---"
# kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite

# echo
# echo "---------------------------------------------"
# echo "--- helm alias template mode with --create-namespace and --dry-run and 'sample-app' label ---"
# hl --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo
# echo "---------------------------------------------"
# echo "--- helm alias insall mode with --create-namespace and --dry-run and 'sample-app' label ---"
# hl --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo
# echo "---------------------------------------------"
# echo "--- helm alias install mode with --create-namespace and 'sample-app' label ---"
# hl --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

# echo
# echo "---------------------------------------------"
# echo "--- kustomize with KubeStellar bindingpolicy output ---"
# kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite --create-bp
