class Pumba < Formula
  desc "Chaos testing tool for Docker"
  homepage "https://github.com/gaia-adm/pumba"
  version "{{ VERSION }}"

  if Hardware::CPU.is_64_bit?
    url "https://github.com/gaia-adm/pumba/releases/download/{{ VERSION }}/pumba_darwin_amd64"
    sha256 "{{ pumba_darwin_amd64_SHA256 }}"
  else
    url "https://github.com/gaia-adm/pumba/releases/download/{{ VERSION }}/pumba_darwin_386"
    sha256 "{{ pumba_darwin_386_SHA256 }}"
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
