
echo "---------------------------------------------"
echo "--- (helm install with --debug mode - not installed previously) --- should label"
echo "---------------------------------------------"
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --debug | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- (helm install without --debug mode) --- should label"
echo "---------------------------------------------"
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo
echo "---------------------------------------------"
echo "--- (helm template run - no installation - installed previously) --- should label"  
helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo
echo "---------------------------------------------"
echo "--- (helm template run - no installation - and uninstalled before running) --- should not label"  
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
helm --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo
echo "---------------------------------------------"
echo "--- (helm --dry-run - no installation - and not previously installed) --- should not label"
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo
echo "---------------------------------------------"
echo "--- (helm install mode - and not previously installed) --- should label"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo
echo "---------------------------------------------"
echo "--- (helm install mode - and previously installed with --dry-run) --- should label"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sealed-secrets -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should apply label, or not, if label not already present"
kubectl --context=kind-kind apply -f ./examples/kubectl/pass | ./labeler -l app.kubernetes.io/part-of=sample-value

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should apply label, or not, if label not already present"
kubectl --context=kind-kind apply -f ./examples/kubectl/pass | ./labeler -l app.kubernetes.io/part-of=sample-value --overwrite

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should apply label"
kubectl --context=kind-kind apply -f ./examples/kubectl/pass | ./labeler -l app.kubernetes.io/part-of=sample-value2 --overwrite

echo "---------------------------------------------"
echo "--- (with error returning from kubectl) --- should fail"
kubectl --context=kind-kind apply -f ./examples/kubectl/fail | ./labeler -l app.kubernetes.io/part-of=sample-value 

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should apply label, or not, if label not already present"
kubectl apply -f ./examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=my-kubectl-app -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should apply label, or not if label already present"
kubectl apply -f ./examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=my-kubectl-app -k ~/.kube/config -c kind-kind --overwrite

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should not apply label since last test applied a label"
kubectl apply -f ./examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=my-kubectl-ap2 -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (kubectl apply -f) --- should apply label since label is not the same, and overwrite is set"
kubectl apply -f ./examples/kubectl/pass/deployment.yml | ./labeler -l app.kubernetes.io/part-of=my-kubectl-ap2 -k ~/.kube/config -c kind-kind --overwrite

echo "---------------------------------------------"
echo "--- (kustomize - 'kubectl -k') --- should not apply if label is present"
kubectl apply -k ./examples/kustomize | ./labeler -l app.kubernetes.io/part-of=my-kustomize-app -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (kustomize - 'kubectl -k') --- should apply if label not the same"
kubectl apply -k ./examples/kustomize | ./labeler -l app.kubernetes.io/part-of=my-kustomize-app -k ~/.kube/config -c kind-kind --overwrite

echo "---------------------------------------------"
echo "--- (kustomize - 'kubectl -k') --- should not apply if label is present"
kubectl apply -k ./examples/kustomize | ./labeler -l app.kubernetes.io/part-of=my-kustomize-ap2 -k ~/.kube/config -c kind-kind

echo "---------------------------------------------"
echo "--- (kustomize - 'kubectl -k') --- should apply"
kubectl apply -k ./examples/kustomize | ./labeler -l app.kubernetes.io/part-of=my-kustomize-ap2 -k ~/.kube/config -c kind-kind --overwrite



echo "---------------------------------------------"
echo "--- helm (local chart) --- should install and label, unless chart is missing and then should give helm's error"
helm --kube-context=kind-kind install my-release-name ./mychart | ./labeler app.kubernetes.io/part-of=my-release-value


echo "--- passing test: ---"
helm --kube-context=kind-kind install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --create-namespace --version 1.2.0 | ./labeler -l app.kubernetes.io/part-of=sample-value

# echo "--- failing test (missing kubeconfig) ---"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig eks.config --context kind-kind

# echo "--- passing test (context and kubeconfig exist) ---"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig ~/.kube/config --context kind-kind

# echo "--- - or - (on ubuntu) ---"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run > exec | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

# echo "--- (note the use of 'exec' to get the command into history) ---"
helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace > exec | ./labeler -l app.kubernetes.io/part-of=sample-value -k ~/.kube/config -c kind-kind

# helm --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --dry-run | ./labeler -l app.kubernetes.io/part-of=sample-value --kubeconfig ~/.kube/config --context kind-kind
helm install nginx oci://ghcr.io/nginxinc/charts/nginx-ingress -n nginx --version 1.2.0 | ./labeler -l app.kubernetes.io/part-of=sample-value


echo
echo "---------------------------------------------"
echo "               ALIAS TESTS"
echo "---------------------------------------------"
alias kl='labeler kubectl'
alias hl='labeler helm'

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - without --overwrite ---"
kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - with --overwrite ---"
kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - with --overwrite ---"
# 
kl create namespace temp
kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - without --overwrite ---"
kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp

echo
echo "---------------------------------------------"
echo "--- kustomize alias with 'sample-app' label without --overwrite ---"
kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default

echo
echo "---------------------------------------------"
echo "--- kustomize alias with 'sample-app' label without --overwrite ---"
kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default


echo
echo "---------------------------------------------"
echo "--- kustomize alias with 'sample' label with --overwrite ---"
kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite

echo
echo "---------------------------------------------"
echo "--- helm alias template mode with --create-namespace and --dry-run and 'sample-app' label ---"
hl --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- helm alias insall mode with --create-namespace and --dry-run and 'sample-app' label ---"
hl --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- helm alias install mode with --create-namespace and 'sample-app' label ---"
hl --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- kustomize with KubeStellar bindingpolicy output ---"
kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite --create-bp
