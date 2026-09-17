.PHONY: build ui test race vet typecheck check smoke test-package release-local release-snapshot
build:
	sh scripts/build.sh
ui:
	npm run build:ui
typecheck:
	npm run typecheck
test:
	go test ./... -count=1 -timeout=120s
race:
	go test -race ./... -count=1 -timeout=120s
vet:
	go vet ./...
check:
	npm run typecheck
	npm run build:ui
	go vet ./...
	go test ./... -count=1 -timeout=120s
	go test -race ./... -count=1 -timeout=120s
	python3 -m unittest discover -s scripts/tests -v
smoke: build
	python3 scripts/smoke-binary.py "bin/nearprod-$$(go env GOOS)-$$(go env GOARCH)" --expected "$$(cat VERSION)"
test-package: build
	python3 scripts/test-package.py "bin/nearprod-$$(go env GOOS)-$$(go env GOARCH)"
# Run npm ci --ignore-scripts before invoking these build/release targets.
release-local: check
	python3 scripts/release.py
# Offline validation only; never publish these snapshots.
release-snapshot:
	python3 scripts/release.py --snapshot --prebuilt-ui
