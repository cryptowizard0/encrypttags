#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
E2E_LOG_PREFIX="encrypttags-e2e"
E2E_LOG_NAME="encrypttags-e2e"
source "$SCRIPT_DIR/e2e-common.sh"

usage() {
	cat <<EOF
Usage: $(basename "$0") [--dry-run] [--help]

Runs the full encrypttags local e2e flow:
  1. recreate Redis container
  2. start hymx node in the background
  3. initialize token and registry
  4. run encrypttags e2e and capture process pid
  5. stop node to write checkpoint
  6. verify checkpoint has no plaintext leak
  7. restart node and verify checkpoint restore decryption

Environment:
  REDIS_CONTAINER       default: hype-vmdocker-redis
  REDIS_IMAGE           default: redis:latest
  NODE_READY_PATTERN    default: recovery node successfully|server is running
  NODE_READY_TIMEOUT    default: 60
  NODE_STOP_TIMEOUT     default: 30
  REDIS_READY_TIMEOUT   default: 30
  LOG_DIR               default: .tmp/encrypttags-e2e/<timestamp>
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
		step "STEP 1/7: Prepare Redis and build node"
		prepare_redis_and_build_node
		result "Redis would be recreated and node binary would be built"
		step "STEP 2/7: Start initial node"
		start_node "initial"
		result "Initial node would be ready"
		step "STEP 3/7: Initialize token and registry"
		run_examples "init" init
		result "Token and registry would be initialized"
		step "STEP 4/7: Run encrypted tag e2e"
		run_examples "encrypttags" encrypttags
		log "+ extract PROCESS pid=<pid>"
		result "Encrypted tag e2e would pass and process pid would be captured"
		step "STEP 5/7: Stop node and validate checkpoint"
		stop_node_for_checkpoint
		run_examples "checkpoint" checkpoint "<pid>"
		result "Checkpoint would be validated"
		step "STEP 6/7: Restart node from checkpoint"
		start_node "restore"
		result "Restored node would be ready"
		step "STEP 7/7: Verify checkpoint restore decrypts encrypted message"
		run_examples "checkpoint-restore" checkpoint-restore "<pid>"
		result "Dry run complete"
		return 0
	fi

	require_e2e_cmds

	mkdir -p "$LOG_DIR"
	log "logs: $LOG_DIR"

	step "STEP 1/7: Prepare Redis and build node"
	prepare_redis_and_build_node
	result "Redis ready and node binary built: $NODE_BIN"

	step "STEP 2/7: Start initial node"
	start_node "initial"

	step "STEP 3/7: Initialize token and registry"
	run_examples "init" init

	step "STEP 4/7: Run encrypted tag e2e"
	run_examples "encrypttags" encrypttags

	local encrypttags_log="$LOG_DIR/examples-encrypttags.log"
	grep -q "E2E encrypted tags passed" "$encrypttags_log"
	grep -q "RAW message encrypted=true plaintext_leaked=false" "$encrypttags_log"
	result "Encrypted tag e2e output verified"

	local process_id
	process_id="$(extract_process_id "$encrypttags_log")"
	if [[ -z "$process_id" ]]; then
		echo "failed to extract process id from $encrypttags_log" >&2
		exit 1
	fi
	result "Captured process id: $process_id"

	step "STEP 5/7: Stop node and validate checkpoint"
	stop_node_for_checkpoint
	run_examples "checkpoint" checkpoint "$process_id"
	grep -q "CHECKPOINT plaintext_leaked=false" "$LOG_DIR/examples-checkpoint.log"
	result "Checkpoint plaintext leak check passed for pid=$process_id"

	step "STEP 6/7: Restart node from checkpoint"
	start_node "restore"

	step "STEP 7/7: Verify checkpoint restore decrypts encrypted message"
	run_examples "checkpoint-restore" checkpoint-restore "$process_id"
	grep -q "CHECKPOINT restore_decrypted=true" "$LOG_DIR/examples-checkpoint-restore.log"

	result "Checkpoint restore decrypt check passed for pid=$process_id"
	result "Automated e2e passed"
	result "Process id: $process_id"
	result "Logs: $LOG_DIR"
}

main "$@"
