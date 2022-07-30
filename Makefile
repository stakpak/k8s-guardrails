
build-webhook:
	pack build guardrails-webhook:$(VERSION) \
	--buildpack paketo-buildpacks/go         \
	--builder paketobuildpacks/builder:tiny  \
	--env BP_GO_TARGETS="./cmd/webhook"

test-webhook:
	go test ./cmd/webhook


build-init:
	pack build guardrails-init:$(VERSION)    \
	--buildpack paketo-buildpacks/go         \
	--builder paketobuildpacks/builder:tiny  \
	--env BP_GO_TARGETS="./cmd/init"
