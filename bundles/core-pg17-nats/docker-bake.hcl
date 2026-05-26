group "default" {
  targets = ["core-pg17-nats"]
}

target "core-pg17-nats" {
  context = "."
  dockerfile = "bundles/core-pg17-nats/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-core-pg17-nats:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
