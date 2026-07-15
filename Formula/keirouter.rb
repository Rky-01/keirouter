# Auto-updated by release.yml on tag v0.1.26. Do not edit manually.
class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "0.1.26"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.26/keirouter_v0.1.26_darwin_arm64.tar.gz"
      sha256 "77fbc69d30418ce1c9db251b0033498ba1dea1c8d8378502010a1f56c019100a"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.26/keirouter_v0.1.26_darwin_amd64.tar.gz"
      sha256 "e3c618bd7aeac02bed7b3eba70e5420be82b3517f26bb53ecdd41930bd0044c0"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.26/keirouter_v0.1.26_linux_arm64.tar.gz"
      sha256 "1656ec77f4d89e2bb24683cc0d3cf23b47af2b73b88c4077e2e427d39ea24379"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.26/keirouter_v0.1.26_linux_amd64.tar.gz"
      sha256 "2900a8ff89e635fb6fe8fefb114ceefa86b76b36df7cde965a0b2c30bcfd3ea8"
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
        keirouter start         # start server on :20180

      Dashboard: http://localhost:20180  (default password: keirouter)
    EOS
  end

  test do
    assert_match "KeiRouter", shell_output("\#{bin}/keirouter --help 2>&1")
  end
end
