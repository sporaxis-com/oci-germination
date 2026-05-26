group "default" {
  targets = ["bundle-ck-allinone"]
}

target "bundle-ck-allinone" {
  context = "."
  dockerfile = "bundles/bundle-ck-allinone/Dockerfile"
  tags = ["ghcr.io/sporaxis-com/ociger-ck-allinone:dev"]
  platforms = ["linux/amd64", "linux/arm64"]
}
