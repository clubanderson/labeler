class Labeler < Formula
    desc "Utility that automates the labeling of resources output from kubectl, kustomize, and helm"
    homepage "https://github.com/clubanderson/labeler"
    url "https://github.com/clubanderson/labeler/releases/download/v0.1.0/labeler-v0.1.0.tar.gz"
    sha256 "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  
    def install
      bin.install "labeler"
    end
  
    test do
      system "#{bin}/labeler", "--version"
    end
  end