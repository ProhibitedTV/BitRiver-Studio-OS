POWERSHELL ?= powershell -ExecutionPolicy Bypass

.DEFAULT_GOAL := help

.PHONY: help install dev dev-api dev-ui lint build package smoke ci openapi

help:
	@echo "BitRiver Studio OS developer targets"
	@echo "  make install   Install Node workspace dependencies and sync Go workspaces"
	@echo "  make dev-api   Run the Go control API locally"
	@echo "  make dev-ui    Run the browser UI locally"
	@echo "  make lint      Run lint and static checks"
	@echo "  make build     Build API and UI artifacts"
	@echo "  make package   Build artifacts and container image scaffold"
	@echo "  make smoke     Run smoke and integration checks against a local API instance"
	@echo "  make ci        Run the PR validation sequence"
	@echo "  make openapi   Generate the shared OpenAPI contract from the Go API package"

install:
	npm install
	go work sync

dev: dev-api

dev-api:
	$(POWERSHELL) -File scripts/dev.ps1 -Target api

dev-ui:
	$(POWERSHELL) -File scripts/dev.ps1 -Target ui

lint:
	$(POWERSHELL) -File scripts/lint.ps1

build:
	$(POWERSHELL) -File scripts/build.ps1

openapi:
	$(POWERSHELL) -File scripts/generate-openapi.ps1

package:
	$(POWERSHELL) -File scripts/package.ps1

smoke:
	$(POWERSHELL) -File scripts/smoke.ps1

ci: lint build smoke
