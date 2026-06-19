COMPOSE ?= podman-compose

.PHONY: build test run-api compose-config compose-up compose-down rmmctl lab-headscale-key lab-bootstrap-token lab-restart-api lab-bootstrap-command lab-enroll-script lab-write-enroll-script lab-sync-nodes lab-node-list deploy-config deploy-up deploy-down deploy-init deploy-headscale-key deploy-bootstrap-token deploy-restart-api deploy-bootstrap-command

build:
	go build ./cmd/rmmctl
	go build ./cmd/rmm-api

test:
	go test ./...

run-api:
	go run ./cmd/rmm-api

compose-config:
	$(COMPOSE) config

compose-up:
	$(COMPOSE) up -d

compose-down:
	$(COMPOSE) down

rmmctl:
	podman run --rm --network host \
		-v $(CURDIR):/src:Z \
		-w /src docker.io/library/golang:1.22 \
		go run ./cmd/rmmctl $(ARGS)

lab-headscale-key:
	./scripts/lab-headscale-key.sh

lab-bootstrap-token:
	./scripts/lab-bootstrap-token.sh

lab-restart-api:
	./scripts/lab-restart-api.sh

lab-bootstrap-command:
	./scripts/lab-bootstrap-command.sh "$(CONTROL_PLANE_URL)"

lab-enroll-script:
	$(MAKE) rmmctl ARGS="node enroll-script --os linux --user lab --ttl 1h --tags tag:rmm-agent --login-server $(LOGIN_SERVER)"

lab-write-enroll-script:
	./scripts/lab-write-enroll-script.sh "$(LOGIN_SERVER)"

lab-sync-nodes:
	$(MAKE) rmmctl ARGS="overlay nodes sync"

lab-node-list:
	$(MAKE) rmmctl ARGS="node list"

deploy-config:
	./scripts/deploy-render.sh
	$(COMPOSE) -f deploy/generated/compose.yml config

deploy-up:
	./scripts/deploy-render.sh
	$(COMPOSE) -f deploy/generated/compose.yml build rmm-api
	$(COMPOSE) -f deploy/generated/compose.yml up -d

deploy-down:
	$(COMPOSE) -f deploy/generated/compose.yml down

deploy-init:
	./scripts/deploy-init.sh

deploy-headscale-key:
	./scripts/deploy-headscale-key.sh

deploy-bootstrap-token:
	./scripts/deploy-bootstrap-token.sh

deploy-restart-api:
	./scripts/deploy-render.sh
	$(COMPOSE) -f deploy/generated/compose.yml build rmm-api
	$(COMPOSE) -f deploy/generated/compose.yml up -d --force-recreate rmm-api

deploy-bootstrap-command:
	./scripts/deploy-bootstrap-command.sh
