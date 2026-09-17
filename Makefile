.PHONY: build ui test race vet typecheck
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
