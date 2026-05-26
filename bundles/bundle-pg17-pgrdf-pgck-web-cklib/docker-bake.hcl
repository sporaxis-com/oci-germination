group "default" {
  targets = ["bundle-pg17-pgrdf-pgck-web-cklib"]
}

target "bundle-pg17-pgrdf-pgck-web-cklib" {
  context = "."
  dockerfile = "bundles/bundle-pg17-pgrdf-pgck-web-cklib/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-web-cklib:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
