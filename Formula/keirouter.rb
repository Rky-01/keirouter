# This file is auto-updated by .github/workflows/release.yml on each tag.
# Manual edits will be overwritten on next release.
# Template: see the "Update Homebrew formula" step in release.yml.

class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "__VERSION__"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v__VERSION__/keirouter_v__VERSION___darwin_arm64.tar.gz"
      sha256 "__SHA256_DARWIN_ARM64__"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v__VERSION__/keirouter_v__VERSION___darwin_amd64.tar.gz"
      sha256 "__SHA256_DARWIN_AMD64__"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v__VERSION__/keirouter_v__VERSION___linux_arm64.tar.gz"
      sha256 "__SHA256_LINUX_ARM64__"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v__VERSION__/keirouter_v__VERSION___linux_amd64.tar.gz"
      sha256 "__SHA256_LINUX_AMD64__"
    end
  end

  def install
    bin.install "keirouter"
    share.install "frontend" => "keirouter/frontend"
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
    assert_match "KeiRouter", shell_output("#{bin}/keirouter --help 2>&1", 2)
  end
end