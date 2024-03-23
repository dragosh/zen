#-----------------------------------------------------------
# Make it
#-----------------------------------------------------------
define BANNER

ooooooo_________________
_____oo__ooooo__oo_ooo__
____oo__oo____o_ooo___o_
___o____ooooooo_oo____o_
_oo_____oo______oo____o_
ooooooo__ooooo__oo____o_
________________________

endef

#MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash
TERM := xterm
INTERACTIVE:=$(shell [ -t 0 ] && echo 1)
.SHELLFLAGS := -o pipefail -euc
.DEFAULT_GOAL := help
ROOT=$(shell pwd)
ENV_FILE := .env
.PHONY: all help setup clean fix test show start stop build/cli banner

## Global bin
echo=$(shell which echo)
sh=$(shell which sh)

-include $(ENV_FILE)

ifneq ("$(wildcard $($(ENV_FILE)))","")
export $(shell grep -v '^#' ${ROOT}/.env | xargs -I '\n')
endif

## Private
_LINE_ = "-----------------------------------------------------------"
# @todo make it based on aliases
_E_CODE_ = $$?

# use the rest as arguments
arg := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
# ...and turn them into do-nothing targets
$(eval $(arg):;@:)


define log_info
	echo "${COLOR_CYAN}[INFO]:${COLOR_NONE}$(1)"
endef

define log_warn
	echo "${COLOR_YELLOW}[WARNING]:${COLOR_NONE}$(1)"
endef

define log_debug
	echo "${COLOR_BLUE}[DEBUG]:${COLOR_NONE}$(1)"
endef

define log_error
	(echo "${COLOR_RED}[ERROR $(strip ${_E_CODE_})]:${COLOR_NONE}$(subst $\",,$(1))"; exit 1)
endef

define log_success
	echo "${COLOR_GREEN}[SUCCESS]:${COLOR_NONE}$(1)"
endef


# -----------------------------------------------------------
# Help tasks
# make help
# -----------------------------------------------------------
help: banner ## Display all make commands
	@echo ${_LINE_}
	@awk 'BEGIN {FS = ":.*##"; \
		printf "Usage: make ${COLOR_CYAN}<command>${COLOR_NONE}\n\n"} /^[a-zA-Z_\/-]+:.*?##/ { \
		printf "${COLOR_CYAN}%-30s${COLOR_NONE} %s\n", $$1, $$2 } /^##@/ { \
		printf "\n\033[1m%s${COLOR_NONE}\n", substr($$0, 5) }'\
		$(MAKEFILE_LIST)
	@echo ${_LINE_}

build/cli: banner ## Build CLI
	@$(call log_info, Build CLI)
	@$(sh) ${ROOT}/scripts/build cli $(arg)

build/appbundle:
	@go run build/macapp.go -assets target/macos -name "zen" -bin app -icon build/icon.png
	@dylibbundler -od -b -x ./zen.app/Contents/MacOS/app  -p @executable_path/../bin/


banner:
	$(info $(BANNER))

# -----------------------------------------------------------
# Colors
# -----------------------------------------------------------
# Color       #define       Value       RGB
# black     COLOR_BLACK       0     0, 0, 0
# red       COLOR_RED         1     max,0,0
# green     COLOR_GREEN       2     0,max,0
# yellow    COLOR_YELLOW      3     max,max,0
# blue      COLOR_BLUE        4     0,0,max
# magenta   COLOR_MAGENTA     5     max,0,max
# cyan      COLOR_CYAN        6     0,max,max
# white     COLOR_WHITE       7     max,max,max

ifdef INTERACTIVE
COLOR_BLACK:=$(shell tput setaf 0)
COLOR_RED:=$(shell tput setaf 1)
COLOR_GREEN:=$(shell tput setaf 2)
COLOR_YELLOW:=$(shell tput setaf 3)
COLOR_BLUE:=$(shell tput setaf 4)
COLOR_MAGENTA:=$(shell tput setaf 5)
COLOR_CYAN:=$(shell tput setaf 6)
COLOR_WHITE:=$(shell tput setaf 7)
COLOR_NONE:=$(shell tput sgr0)

UNDER:=$(shell tput smul)
BOLD:=$(shell tput bold)
endif

ifeq ($(NO_COLOR),true)
override COLOR_BLACK=$(COLOR_NONE)
override COLOR_YELLOW=$(COLOR_NONE)
override COLOR_RED=$(COLOR_NONE)
override COLOR_GREEN= $(COLOR_NONE)
override COLOR_BLUE=$(COLOR_NONE)
override COLOR_MAGENTA=$(COLOR_NONE)
override COLOR_CYAN=$(COLOR_NONE)
override COLOR_WHITE=$(COLOR_NONE)
endif
