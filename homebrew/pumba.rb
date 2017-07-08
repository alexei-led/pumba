require "language/go"

class Pumba < Formula
  desc "Chaos testing tool for Docker"
  homepage "https://github.com/gaia-adm/pumba"
  version "0.4.4"
  url "https://github.com/gaia-adm/pumba/archive/0.4.4.tar.gz"
  sha256 "eba3d0d66944cd408348f52df5cc7c767414261a3104c6c868c5a1ac761c376d"
  head "https://github.com/gaia-adm/pumba.git"

  bottle do
    cellar :any_skip_relocation
    sha256 "8b39861136f99025c7fb9ea3ee4a8143c890ef982361152469b785a2dacb9534" => :sierra
  end

  depends_on "go" => :build
  depends_on "glide" => :build

  def install
    ENV["GOPATH"] = buildpath
    ENV["GLIDE_HOME"] = HOMEBREW_CACHE/"glide_home/#{name}"
    ENV["CGO_ENABLED"] = "0"

    ENV["GOPATH"] = buildpath
    ENV["GLIDE_HOME"] = HOMEBREW_CACHE/"glide_home/#{name}"
    pumbapath = buildpath/"src/github.com/gaia-adm/pumba"
    pumbapath.install Dir["{*,.git}"]

    ldflags = "-X main.Version=#{version} -X main.GitCommit=92d78a0 -X main.GitBranch=master -X main.BuildTime=2017-07-08_09:05_GMTb"

    cd pumbapath do
      system "glide", "install", "-v"
      system "go", "build", "-v", "-o", "dist/pumba", "-ldflags", ldflags
      bin.install "dist/pumba"
    end
  end

  test do
    system "#{bin}/pumba", "--version"
  end
end
