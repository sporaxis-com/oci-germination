group "default" {
  targets = ["bundle-pg18-pgrdf-pgck-nats-micro"]
}

target "bundle-pg18-pgrdf-pgck-nats-micro" {
  context = "."
  dockerfile = "bundles/bundle-pg18-pgrdf-pgck-nats-micro/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-pg18-pgrdf-pgck-nats-micro:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
