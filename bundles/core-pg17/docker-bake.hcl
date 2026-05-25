group "default" {
  targets = ["core-pg17"]
}

target "core-pg17" {
  context = "."
  dockerfile = "bundles/core-pg17/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-core-pg17-min:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
