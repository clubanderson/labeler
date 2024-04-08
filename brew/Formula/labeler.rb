class Labeler < Formula
  desc "Utility that automates the labeling of resources output from kubectl, kustomize, and helm"
  homepage "https://github.com/clubanderson/hackathonlabeler"
  url "https://github.com/clubanderson/hackathonlabeler/releases/download/v0.2.0/labeler"
  # sha256 "be174998d8a312930897bae18342b6721f060aba6748b527506c95e3011fa535"

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
      echo 'alias kl="labeler kubectl"' >> ~/.bashrc
      echo 'alias hl="labeler helm"' >> ~/.bashrc
      source ~/.bashrc
      
      For Zsh users:
      echo 'alias kl="labeler kubectl"' >> ~/.zshrc
      echo 'alias hl="labeler helm"' >> ~/.zshrc
      source ~/.zshrc

      Then just use `kl` or `hl` in place of `kubectl` or `helm` respectively. Add -l or --label to the end of the command to label ALL of the resources you apply.

      example (kubectl):
        
        kl apply -f examples/kubectl/pass -l app.kubernetes.io/part-of=sample --context=kind-kind --namespace=default --overwrite

      example (kustomize):

        kl apply -k examples/kustomize -l app.kubernetes.io/part-of=sample-app --context=kind-kind --namespace=temp --overwrite

      example (helm):

        hl --kube-context=kind-kind template sealed-secrets sealed-secrets/sealed-secrets -n sealed-secrets --create-namespace --label=app.kubernetes.io/part-of=sample-app --dry-run; helm --kube-context=kind-kind uninstall sealed-secrets -n sealed-secrets
      
      For more information, visit the readme at https://github.com/clubanderson/labeler
    EOS
  end
end