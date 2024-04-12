
print_error() {
    printf "\033[1;31m%s\033[0m\n" "$1"
}

print_success() {
    printf "\033[1;32m%s\033[0m\n" "$1"
}

test_number=1

echo
echo "---------------------------------------------"
echo "               ALIAS TESTS"
echo "---------------------------------------------"
alias k='labeler kubectl'
alias h='labeler helm'

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - without --overwrite ---"
if ! k apply -f ../examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default; then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - with --overwrite ---"
if ! k apply -f ../examples/kubectl/pass --label=app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite; then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - with --overwrite ---"
# 
k create namespace temp
if ! k apply -f ../examples/kubectl/pass --label=app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite; then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kubectl alias with 'sample' label - without --overwrite ---"
if ! k apply -f ../examples/kubectl/pass --label=app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi 
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kustomize alias with 'sample-app' label without --overwrite ---"
if ! k apply -k ../examples/kustomize --label=app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kustomize alias with 'sample-app' label without --overwrite ---"
if ! k apply -k ../examples/kustomize --label=app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=default;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kustomize alias with 'sample' label with --overwrite ---"
if ! k apply -k ../examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=temp --overwrite;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi
((test_number++))

echo
echo "---------------------------------------------"
echo "--- helm alias template mode with --create-namespace and --dry-run and 'sample-app' label ---"
if ! h --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))
h --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- helm alias insall mode with --create-namespace and --dry-run and 'sample-app' label ---"
if ! h --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- helm alias install mode with --create-namespace and 'sample-app' label ---"
if ! h --kube-context=kind-kind install sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))
helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets

echo
echo "---------------------------------------------"
echo "--- kustomize with KubeStellar bindingpolicy output ---"
if ! k apply -k ../examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite --l-bp-name=newbp;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kustomize with debug output ---"
if ! k apply -k ../examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite --l-debug;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kustomize with help output ---"
if ! k apply -k ../examples/kustomize -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite --l-help;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++))

echo
echo "---------------------------------------------"
echo "--- kubectl log - works! ---"
if ! k logs deployment.apps/coredns -n kube-system;then
    print_error "test $test_number: ERROR"
else
    print_success "test $test_number: SUCCESS"
fi  
((test_number++)) 

# echo
# echo "---------------------------------------------"
# echo "--- kubectl log (followed) - works! ---"
# k logs deployment.apps/coredns -n kube-system -f

# echo
# echo "---------------------------------------------"
# echo "--- kubectl edit - works! ---"
# k edit deployment.apps/coredns -n kube-system          