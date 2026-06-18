.PHONY: build test run-api compose-config compose-up compose-down rmmctl lab-headscale-key lab-restart-api lab-enroll-script lab-write-enroll-script lab-sync-nodes lab-node-list

build:
	go build ./cmd/rmmctl
	go build ./cmd/rmm-api

test:
	go test ./...

run-api:
	go run ./cmd/rmm-api

compose-config:
	podman compose config

compose-up:
	podman compose up -d

compose-down:
	podman compose down

rmmctl:
	podman run --rm --network host \
		-v $(CURDIR):/src:Z \
		-w /src docker.io/library/golang:1.22 \
		go run ./cmd/rmmctl $(ARGS)

lab-headscale-key:
	./scripts/lab-headscale-key.sh

lab-restart-api:
	./scripts/lab-restart-api.sh

lab-enroll-script:
	$(MAKE) rmmctl ARGS="node enroll-script --os linux --user lab --ttl 1h --tags tag:rmm-agent --login-server $(LOGIN_SERVER)"

lab-write-enroll-script:
	./scripts/lab-write-enroll-script.sh "$(LOGIN_SERVER)"

lab-sync-nodes:
	$(MAKE) rmmctl ARGS="overlay nodes sync"

lab-node-list:
	$(MAKE) rmmctl ARGS="node list"
