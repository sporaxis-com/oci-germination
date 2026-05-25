group "default" {
  targets = ["bundle-pg17-pgrdf-pgck"]
}

target "bundle-pg17-pgrdf-pgck" {
  context = "."
  dockerfile = "bundles/bundle-pg17-pgrdf-pgck/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
