# Auto-updated by release.yml on tag v0.1.21. Do not edit manually.
class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "0.1.21"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.21/keirouter_v0.1.21_darwin_arm64.tar.gz"
      sha256 "34411e980cdbac92536c5e26d6ec63a1469980996909bd7f6fbb92f76919d6b1"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.21/keirouter_v0.1.21_darwin_amd64.tar.gz"
      sha256 "20d75339fd4eee77f71cd0acb66f0f44a5f9d48ac36287d601aa9a2dad6cd9e8"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.21/keirouter_v0.1.21_linux_arm64.tar.gz"
      sha256 "456fafb08519f7796feed3c38057fd95dc9edb985103d93fb475eec952953ecf"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.21/keirouter_v0.1.21_linux_amd64.tar.gz"
      sha256 "8a97d9f423edc4c9b802946fa538a039356e59e137b0e14f21c0eb8e9a8c0571"
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
