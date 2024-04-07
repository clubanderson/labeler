class Labeler < Formula
    desc "Utility that automates the labeling of resources output from kubectl, kustomize, and helm"
    homepage "https://github.com/clubanderson/hackathon-labeler"
    url "https://github.com/clubanderson/hackathon-labeler/releases/download/v0.1.0/labeler"
    sha256 "be174998d8a312930897bae18342b6721f060aba6748b527506c95e3011fa535"
  
    def install
      bin.install "labeler"
    end
  
    test do
      system "#{bin}/labeler", "--version"
    end
  end