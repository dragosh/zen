#!/usr/bin/env bash
# -----------------------------------------------------------
# ☢️ : THIS FILE IS INCLUDES in some docker images
# -----------------------------------------------------------

set -eu
export TERM="xterm"

ROOT=${PWD}
ENV_FILE=$ROOT/.env

####################################################################
DEBUG=${DEBUG:-true}
LOG_LEVEL=${LOG_LEVEL:-warning} # TODO
COLOR_NONE=$(tput sgr0)
_WITH_ERROR_=0

## Disable color by default
if [[ ${NO_COLOR+x} ]]; then
	COLOR_BLACK=$COLOR_NONE
	COLOR_YELLOW=$COLOR_NONE
	COLOR_RED=$COLOR_NONE
	COLOR_GREEN=$COLOR_NONE
	COLOR_BLUE=$COLOR_NONE
	COLOR_MAGENTA=$COLOR_NONE
	COLOR_CYAN=$COLOR_NONE
	COLOR_WHITE=$COLOR_NONE
else
	COLOR_BLACK=$(tput setaf 0)
	COLOR_RED=$(tput setaf 1)
	COLOR_GREEN=$(tput setaf 2)
	COLOR_YELLOW=$(tput setaf 3)
	COLOR_BLUE=$(tput setaf 4)
	COLOR_MAGENTA=$(tput setaf 5)
	COLOR_CYAN=$(tput setaf 6)
	COLOR_WHITE=$(tput setaf 7)

	UNDER=$(tput smul)
	BOLD=$(tput bold)
fi

_log_info() {
	echo "${COLOR_CYAN}[INFO]:${COLOR_NONE} $@"
}

_log_warn() {
	echo "${COLOR_YELLOW}[WARNING]:${COLOR_NONE} $@"
}

_bot() {
	echo "${COLOR_BLUE}\[..]/:${COLOR_NONE} $@"
}

# example _log_sep "--------------------------"
_log_sep() {
	local sep=${1:-""}
	echo $sep
}

_log_debug() {
	if [ $DEBUG == "true" ]; then
		echo "${COLOR_BLUE}[DEBUG]:${COLOR_NONE} $@"
	fi
}

_log_error() {
	echo "${COLOR_RED}[ERROR]:${COLOR_NONE} $1"
	#_WITH_ERROR_=1
}

_log_success() {
	echo "${COLOR_GREEN}[SUCCESS]:${COLOR_NONE} $@"
}

_fail() {
	echo "${COLOR_RED}[ERROR]:${COLOR_NONE}: ${1}" >&2
	exit "${2-1}"
}

_checksum() {
	echo $(find $1 -type f -type f -print0 | sort -z | xargs -0 openssl sha1 | openssl sha1 | cut -d ' ' -f 1)
}

_confirm() {
  _log_info $1
  read -r -p "Continue? (y|yes|Y) " confirm
  if [[ ! $confirm =~ (y|yes|Y) ]]; then
   exit 1
  fi
}

_reload_env() {
	if [ ! -f $ENV_FILE ]; then
		_fail "Unable to find ${COLOR_RED}.env${COLOR_NONE} file! Please run first '${COLOR_GREEN}make setup${COLOR_NONE}' command"
	fi
	if [ "$(uname)" = "Darwin" ]; then
    export $(grep -v '^#' $ENV_FILE | xargs -I '\n')
  elif [ "$(expr substr $(uname -s) 1 5)" = "Linux" ]; then
    export $(grep -v '^#' $ENV_FILE | xargs -d '\n')
  elif [ "$(expr substr $(uname -s) 1 10)" = "MINGW64_NT" ]; then
    echo "TODO"
  fi
}

_try() {
    "$@"
    local status=$?
    if (( status != 0 )); then
        _fail "error with $1" >&2
    fi
    return $status
}

_check_dependencies() {
	local deps=("$@")
	_log_debug "Check main dependencies: ${deps[@]}"
	for dep in "${deps[@]}"
	do
		which $dep >/dev/null 2>&1 || _fail "Unable to find ${COLOR_RED}$dep${COLOR_NONE} binary in $PATH"
	done
}

# check to see if a file is being run or sourced from another script
_is_sourced() {
	# https://unix.stackexchange.com/a/215279
	[ "${#FUNCNAME[@]}" -ge 2 ] \
		&& [ "${FUNCNAME[0]}" = '_is_sourced' ] \
		&& [ "${FUNCNAME[1]}" = 'source' ]
}