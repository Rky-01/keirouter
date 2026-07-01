# Auto-updated by release.yml on tag v0.1.23. Do not edit manually.
class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "0.1.23"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.23/keirouter_v0.1.23_darwin_arm64.tar.gz"
      sha256 "15ae58f14fd335fcdacb9b7b0d463b8c791ea9f31b9fff8a748b6b9ca384bbfd"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.23/keirouter_v0.1.23_darwin_amd64.tar.gz"
      sha256 "366e25a5bf2cf359b5bad89507d8110f8931510498aca45e26e8fa1059af2531"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.23/keirouter_v0.1.23_linux_arm64.tar.gz"
      sha256 "4be577f1df24bb128570facea5e2eed070bcebd2fa145095c1a8ddf35d11180f"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.23/keirouter_v0.1.23_linux_amd64.tar.gz"
      sha256 "93435b939de4d90728bed5a8e98fe2a97f843aaff4040d78d620e13e28a2cd96"
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
