# Reference Homebrew formula for fort-native (backlog AO-043).
# In practice GoReleaser generates and pushes this to tobsai/homebrew-tap on
# release; this checked-in copy documents the shape and lets you `brew install
# --build-from-source ./Formula/fort.rb` locally.
class Fort < Formula
  desc "Deterministic agent orchestration — route, run, and gate agent CLIs natively"
  homepage "https://github.com/tobsai/fort"
  license "MIT"
  head "https://github.com/tobsai/fort.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(
      ldflags: "-s -w -X main.version=#{version}",
      output: bin/"fort",
    ), "./cmd/fort"
  end

  test do
    assert_match "fort", shell_output("#{bin}/fort version")
  end
end
