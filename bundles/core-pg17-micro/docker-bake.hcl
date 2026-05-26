group "default" {
  targets = ["core-pg17-micro"]
}

target "core-pg17-micro" {
  context = "."
  dockerfile = "bundles/core-pg17-micro/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-core-pg17-micro:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
