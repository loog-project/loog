SHELL := /bin/bash

PLUGIN_SRC ?= ./compat/k9s/plugins.yaml
PLUGIN_NAME ?= loog-plugin.yaml

BIN_DIR := /usr/local/bin
KUBECTL_OBS := $(BIN_DIR)/kubectl-observe
KUBECTL_OBS_COMPLETION := $(BIN_DIR)/kubectl_complete-observe

# ----

.PHONY: build
build:
	@echo "Building loog..."
	@go install .
	@echo "Done!"

# -----

.PHONY: link-kubectl
link-kubectl: build
	@echo "Installing kubectl-observe plugin..."
	
	@command -v loog >/dev/null 2>&1 || { echo "error: loog not found in PATH."; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "error: kubectl not found in PATH."; exit 1; }

	@sudo ln -sf "$$(command -v loog)" "$(KUBECTL_OBS)"
	@sudo chmod +x "$(KUBECTL_OBS)"

	@echo "Installing kubectl-observe completion script..."
	@sudo ln -sf "$(PWD)/compat/kubectl/kubectl_complete-observe" "$(KUBECTL_OBS_COMPLETION)"
	@sudo chmod +x "$(KUBECTL_OBS_COMPLETION)"
	
	@echo "Installation complete! Use 'kubectl observe'."

.PHONY: unlink-kubectl
unlink-kubectl:
	@echo "Unlinking kubectl-observe plugin..."
	@sudo rm -f "$(KUBECTL_OBS)" "$(KUBECTL_OBS_COMPLETION)"
	@echo "Unlinked."

# ----

define resolve_k9s_base
{ \
  unix_base="$${HOME}/.config/k9s"; \
  mac_base="$${HOME}/Library/Application Support/k9s"; \
  if [[ -d "$$mac_base" ]]; then \
    echo "$$mac_base"; \
  elif [[ -d "$$unix_base" ]]; then \
    echo "$$unix_base"; \
  fi; \
}
endef

.PHONY: link-k9s
link-k9s:
	@echo "Linking k9s plugin (loog)..."
	
	@[[ -f "$(PLUGIN_SRC)" ]] || { echo "error: Plugin source '$(PLUGIN_SRC)' not found."; exit 1; }

	@K9S_BASE="$$(bash -c '$(resolve_k9s_base)')"; \
		if [[ -z "$$K9S_BASE" ]]; then \
	 		echo "warn: No k9s config directory found; nothing to link."; \
	  	exit 0; \
		fi; \
		mkdir -p "$$K9S_BASE/plugins"; \
		ln -sf "$(abspath $(PLUGIN_SRC))" "$$K9S_BASE/plugins/$(PLUGIN_NAME)"; \
		echo "Linked $(PLUGIN_SRC) to $$K9S_BASE/plugins/$(PLUGIN_NAME)"

.PHONY: unlink-k9s
unlink-k9s:
	@echo "Unlinking k9s plugin (loog)..."

	@K9S_BASE="$$(bash -c '$(resolve_k9s_base)')"; \
		if [[ -z "$$K9S_BASE" ]]; then \
	 		echo "warn: No k9s config directory found; nothing to unlink."; \
	  	exit 0; \
		fi; \
		rm -f "$$K9S_BASE/plugins/$(PLUGIN_NAME)"; \
		echo "Unlinked k9s plugin (loog)."

# -----

.PHONY: install-all
install-all: build link-kubectl link-k9s
	@echo "Installation complete! (loog, kubectl, k9s)"

.PHONY: uninstall-all
uninstall-all: unlink-kubectl unlink-k9s
	@echo "Removed installed links."
	@echo "Delete the loog binary if you want to uninstall it completely."
