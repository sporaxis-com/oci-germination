group "default" {
  targets = ["core-pg17-nats-micro"]
}

target "core-pg17-nats-micro" {
  context = "."
  dockerfile = "bundles/core-pg17-nats-micro/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-core-pg17-nats-micro:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
