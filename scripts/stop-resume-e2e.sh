#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
E2E_LOG_PREFIX="stop-resume-e2e"
E2E_LOG_NAME="stop-resume-e2e"
source "$SCRIPT_DIR/e2e-common.sh"

ENCRYPTTAGS_ADMIN_URL="${ENCRYPTTAGS_ADMIN_URL:-http://127.0.0.1:8081}"
export ENCRYPTTAGS_ADMIN_URL

usage() {
	cat <<EOF
Usage: $(basename "$0") [--dry-run] [--help]

Runs the stop/resume local e2e flow:
  1. recreate Redis container
  2. start hymx node in the background
  3. initialize token and registry
  4. spawn an echo process and capture process pid
  5. stop/resume the spawned process through admin API

Environment:
  REDIS_CONTAINER       default: hype-vmdocker-redis
  REDIS_IMAGE           default: redis:latest
  ENCRYPTTAGS_ADMIN_URL default: http://127.0.0.1:8081
  NODE_READY_PATTERN    default: recovery node successfully|server is running
  NODE_READY_TIMEOUT    default: 60
  NODE_STOP_TIMEOUT     default: 30
  REDIS_READY_TIMEOUT   default: 30
  LOG_DIR               default: .tmp/stop-resume-e2e/<timestamp>
  CLEAN_REDIS_ON_EXIT   default: 0
  NODE_BIN              default: build/hymx-node
EOF
}

for arg in "$@"; do
	case "$arg" in
	--dry-run)
		DRY_RUN=1
		;;
	--help|-h)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $arg" >&2
		usage >&2
		exit 2
		;;
	esac
done

main() {
	if [[ "$DRY_RUN" == "1" ]]; then
		step "STEP 1/5: Prepare Redis and build node"
		prepare_redis_and_build_node
		log "+ ENCRYPTTAGS_ADMIN_URL=$ENCRYPTTAGS_ADMIN_URL"
		result "Redis would be recreated and node binary would be built"
		step "STEP 2/5: Start node"
		start_node "initial"
		result "Node would be ready"
		step "STEP 3/5: Initialize token and registry"
		run_examples "init" init
		result "Token and registry would be initialized"
		step "STEP 4/5: Spawn echo process"
		run_examples "spawn-echo" spawn-echo
		log "+ extract PROCESS pid=<pid>"
		result "Echo process would be spawned and process pid would be captured"
		step "STEP 5/5: Stop/resume spawned process"
		run_examples "stop-resume" stop-resume "<pid>"
		result "Dry run complete"
		return 0
	fi

	require_e2e_cmds

	mkdir -p "$LOG_DIR"
	log "logs: $LOG_DIR"

	step "STEP 1/5: Prepare Redis and build node"
	prepare_redis_and_build_node
	result "Redis ready and node binary built: $NODE_BIN"
	result "Admin API URL: $ENCRYPTTAGS_ADMIN_URL"

	step "STEP 2/5: Start node"
	start_node "initial"

	step "STEP 3/5: Initialize token and registry"
	run_examples "init" init

	step "STEP 4/5: Spawn echo process"
	run_examples "spawn-echo" spawn-echo

	local spawn_log="$LOG_DIR/examples-spawn-echo.log"
	grep -q "SPAWN_ECHO passed" "$spawn_log"
	result "Echo process spawn output verified"

	local process_id
	process_id="$(extract_process_id "$spawn_log")"
	if [[ -z "$process_id" ]]; then
		echo "failed to extract process id from $spawn_log" >&2
		exit 1
	fi
	result "Captured process id: $process_id"

	step "STEP 5/5: Stop/resume spawned process"
	run_examples "stop-resume" stop-resume "$process_id"
	grep -q "STOP_RESUME resume_decrypted=true" "$LOG_DIR/examples-stop-resume.log"

	result "Stop/resume check passed for pid=$process_id"
	result "Automated stop/resume e2e passed"
	result "Process id: $process_id"
	result "Logs: $LOG_DIR"
}

main "$@"
