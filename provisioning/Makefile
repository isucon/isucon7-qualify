# vi: set ts=4 sw=4 sts=0 noet :

SHELL := /bin/bash
default: help

PACKER_DEBUG := $(if $(PACKER_DEBUG), -debug)
PACKER_VARIABLES_FILE := packer_variables.json
ALLINONE_SERVER_MANIFEST := manifest_allinone.json
BUILD_SERVER_MANIFEST := manifest_bench.json
QUALIFIER_SERVER_MANIFEST := manifest_qualifier.json

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Environment Variables:"
	@echo "  PACKER_DEBUG:  Debug(disable paralleliation and stop each step) mode enabled for builds"
	@echo "  PACKER_LOG:    verbose mode."
	@echo ""
	@echo "ex:"
	@echo "  PACKER_LOG=1 PACKER_DEBUG=1 make build-qualifier-vagrant"

create-ova:
	@if [[ -f misc/create-ova/ubuntu1604.ova ]]; then \
	  echo "Skip create ovafile"; \
	else \
	  misc/create-ova/create-ova.sh; \
	fi
	
build-allinone-null: ## build a Qualifier Server with benchmark program by null - ssh and provisioning only (for debug)
	packer build -only=null -var-file=$(PACKER_VARIABLES_FILE) $(PACKER_DEBUG) $(ALLINONE_SERVER_MANIFEST)

build-allinone-vagrant: create-ova ## build a Qualifier with benchmark program Server Vagrant Box
	packer build -only=virtualbox-ovf $(PACKER_DEBUG) $(ALLINONE_SERVER_MANIFEST)

build-allinone-sakura-tk1a: ## build a Qualifier with benchmark program Server Archive for SAKURA Cloud
	packer build -only=sakura-tk1a $(PACKER_DEBUG) $(ALLINONE_SERVER_MANIFEST)

build-bench-null: ## build a Bench Server by null - ssh and provisioning only (for debug)
	packer build -only=null -var-file=$(PACKER_VARIABLES_FILE) $(PACKER_DEBUG) $(BUILD_SERVER_MANIFEST)

build-bench-vagrant: create-ova ## build a Bench Server Vagrant Box
	packer build -only=virtualbox-ovf $(PACKER_DEBUG) $(BUILD_SERVER_MANIFEST)

build-qualifier-null: ## build a Qualifier Server by null - ssh and provisioning only (for debug)
	packer build -only=null -var-file=$(PACKER_VARIABLES_FILE) $(PACKER_DEBUG) $(QUALIFIER_SERVER_MANIFEST)

build-qualifier-vagrant: create-ova ## build a Qualifier Server Vagrant Box
	packer build -only=virtualbox-ovf $(PACKER_DEBUG) $(QUALIFIER_SERVER_MANIFEST)

# https://github.com/Respect/samples/blob/master/Makefile#L295
# Re-usable target for yes no prompt. Usage: make .prompt-yesno message="Is it yes or no?"
# Will exit with error if not yes
.prompt-yesno:
	@exec 9<&0 0</dev/tty; \
	echo "$(message) [y/N]:"; \
	read -r -t 10 -n 3 yn; \
	exec 0<&9 9<&-; \
	if [[ -z $$yn ]]; then \
		echo "Please input y(es) or n(o)."; \
		exit 1; \
	else \
		if [[ $$yn =~ ^[yY] ]]; then \
			echo "continue" >&2; \
		else \
			echo "abort." >&2; \
			exit 1; \
		fi; \
	fi

.PHONY: all help create-ova build-bench-null build-bench-vagrant build-qualifierQualifier-null build-qualifier-vagrant .prompt-yesno

