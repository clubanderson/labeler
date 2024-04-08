class Labeler < Formula
  desc "Utility that automates the labeling of resources output from kubectl, kustomize, and helm"
  homepage "https://github.com/clubanderson/labeler"
  url "https://github.com/clubanderson/labeler/releases/download/v0.2.0/labeler"
  sha256 "ea8a03f413c0f22b172ed9257156ea77195bf7ebaefc3acd3775fcba9343fa35"

  def install
    bin.install "labeler"
  end

  test do
    system "#{bin}/labeler", "--version"
  end

  # After installing, create aliases for convenience
  def caveats
    <<~EOS
      To make using labeler more convenient, consider creating aliases:
      \e[33malias kl="labeler kubectl"\e[0m
      \e[33malias hl="labeler helm"\e[0m

      (if you want these to be permanent, add these to your shell profile, e.g. ~/.bashrc or ~/.zshrc, then source it)

      Then just use `kl` or `hl` in place of `kubectl` or `helm` respectively. Add -l or --label= to the end of the command to label ALL of the resources you apply.

      example (kubectl):
        
         kl apply -f examples/kubectl/pass \e[33m-l app.kubernetes.io/part-of=sample\e[0m --context=kind-kind --namespace=default --overwrite

      example (kustomize):

         kl apply -k examples/kustomize \e[33m-l app.kubernetes.io/part-of=sample-app\e[0m --context=kind-kind --namespace=temp --overwrite

      example (helm):

         hl --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace \e[33m--label=app.kubernetes.io/part-of=sample-app\e[0m --dry-run   
      
      For more information, visit the readme at \e[33mhttps://github.com/clubanderson/labeler\e[0m
    EOS
  end
end