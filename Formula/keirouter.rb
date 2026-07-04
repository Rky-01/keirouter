# Auto-updated by release.yml on tag v0.1.24. Do not edit manually.
class Keirouter < Formula
  desc "AI API router — unified gateway for 20+ LLM providers with fallback, caching, and dashboard"
  homepage "https://github.com/mydisha/keirouter"
  version "0.1.24"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.24/keirouter_v0.1.24_darwin_arm64.tar.gz"
      sha256 "09886a1259980bfee14990f003338c52d9ef2e29363450a934b3599f8992853a"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.24/keirouter_v0.1.24_darwin_amd64.tar.gz"
      sha256 "0a62325c236d231d98a1996ec964e76c21789279f99ba122130ce2cd0e35876e"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.24/keirouter_v0.1.24_linux_arm64.tar.gz"
      sha256 "d92614d2699b819010efbc9929816a69463217f67ef2d76c6bc9403adf3805ac"
    else
      url "https://github.com/mydisha/keirouter/releases/download/v0.1.24/keirouter_v0.1.24_linux_amd64.tar.gz"
      sha256 "1716d70bcc44a37cb4f4813f0f15c65b6c2869e64aef292a57c652e193c03bd2"
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
