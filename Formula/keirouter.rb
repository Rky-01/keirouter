# Auto-updated by release.yml on tag v0.1.19. Do not edit manually.
class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "0.1.19"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.19/keirouter_v0.1.19_darwin_arm64.tar.gz"
      sha256 "8db43180d060f3998fadecf4eca1752eab79c5672064c3e1fc9befa8998d2ba7"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.19/keirouter_v0.1.19_darwin_amd64.tar.gz"
      sha256 "b0b87d5ecae8f20623d09c955dba842141115cad34161c7b7a7553fb0f0fb916"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.19/keirouter_v0.1.19_linux_arm64.tar.gz"
      sha256 "252863e7a2b85cf9b3fce125ac3a78cb7c210a643d72e5a207d98286891a9a38"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.19/keirouter_v0.1.19_linux_amd64.tar.gz"
      sha256 "db5cd2acffe51755d8ebce267a5d485d73bfab5f3bf9d249515182172e6f3ad3"
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
