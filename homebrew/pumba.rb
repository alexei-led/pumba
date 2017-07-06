class Pumba < Formula
  desc "Chaos testing tool for Docker"
  homepage "https://github.com/gaia-adm/pumba"
  version "0.4.2"

  if Hardware::CPU.is_64_bit?
    url "https://github.com/gaia-adm/pumba/releases/download/0.4.2/pumba_darwin_amd64"
    sha256 "af4439a2e94dbc425b1fc13caed6be038d3591961a95ede3a60dfb5be2a6e9ab"
  else
    url "https://github.com/gaia-adm/pumba/releases/download/0.4.2/pumba_darwin_386"
    sha256 "92c45beb2275ba0ee8d81c03eb46f3251fab4257999082ccfe19ab05e73760c7"
  end

  bottle :unneeded

  def install
    if Hardware::CPU.is_64_bit?
      bin.install "pumba_darwin_amd64" => "pumba"
    else
      bin.install "pumba_darwin_386" => "pumba"
    end
  end

  test do
    system "#{bin}/pumba", "--version"
  end
end
