group "default" {
  targets = ["bundle-pg17-pgrdf-pgck-nats"]
}

target "bundle-pg17-pgrdf-pgck-nats" {
  context = "."
  dockerfile = "bundles/bundle-pg17-pgrdf-pgck-nats/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
