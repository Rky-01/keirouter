# Auto-updated by release.yml on tag v0.1.6. Do not edit manually.
class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "0.1.6"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.6/keirouter_v0.1.6_darwin_arm64.tar.gz"
      sha256 "545190271dbe1f6e6810a5590cdf7e23c423d35220b662c06f738f866e65a28e"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.6/keirouter_v0.1.6_darwin_amd64.tar.gz"
      sha256 "d1ea34d02824cdfd23682aad6de6bca2d2e121e6a2211cc8cb1326b5181dd42a"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.6/keirouter_v0.1.6_linux_arm64.tar.gz"
      sha256 "2371884c4d6a263ae6754e4d1a1b600b7f2821a0e5422d37b10102b291dbf42d"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.6/keirouter_v0.1.6_linux_amd64.tar.gz"
      sha256 "c1d2984f2c9b98b166e7d9c7c48b23d3275acbfda493fa8ab7b0db23f941061e"
    end
  end

  def install
    bin.install "keirouter"
    (share/"keirouter").install "frontend"
  end

  def caveats
    <<~EOS
      Quick start:
        keirouter -bootstrap    # create your first API key
        keirouter               # start server on :20180

      Dashboard: http://localhost:20180  (default password: keirouter)
    EOS
  end

  test do
    assert_match "KeiRouter", shell_output("\#{bin}/keirouter --help 2>&1", 2)
  end
end
