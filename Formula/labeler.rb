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
      
      For Bash users:
         \e[33mecho 'alias kl="labeler kubectl"'\e[0m >> ~/.bashrc
         \e[33echo 'alias hl="labeler helm"'\e[0m >> ~/.bashrc
         \e[33source ~/.bashrc\e[0m
      
      For Zsh users:
         \e[33echo 'alias kl="labeler kubectl"'\e[0m >> ~/.zshrc
         \e[33echo 'alias hl="labeler helm"'\e[0m >> ~/.zshrc
         \e[33source ~/.zshrc\e[0m

      Then just use `kl` or `hl` in place of `kubectl` or `helm` respectively. Add -l or --label to the end of the command to label ALL of the resources you apply.

      example (kubectl):
        
         kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite

      example (kustomize):

         kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=temp --overwrite

      example (helm):

         hl --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run   
      
      For more information, visit the readme at https://github.com/clubanderson/labeler
    EOS
  end
end