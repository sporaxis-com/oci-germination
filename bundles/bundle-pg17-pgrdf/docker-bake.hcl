group "default" {
  targets = ["bundle-pg17-pgrdf"]
}

target "bundle-pg17-pgrdf" {
  context = "."
  dockerfile = "bundles/bundle-pg17-pgrdf/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-pg17-pgrdf:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
