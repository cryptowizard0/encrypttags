#!/usr/bin/env bash

E2E_COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$E2E_COMMON_DIR/.." && pwd -P)"

REDIS_CONTAINER="${REDIS_CONTAINER:-hype-vmdocker-redis}"
REDIS_IMAGE="${REDIS_IMAGE:-redis:latest}"
NODE_READY_PATTERN="${NODE_READY_PATTERN:-recovery node successfully|server is running}"
NODE_READY_TIMEOUT="${NODE_READY_TIMEOUT:-60}"
NODE_STOP_TIMEOUT="${NODE_STOP_TIMEOUT:-30}"
REDIS_READY_TIMEOUT="${REDIS_READY_TIMEOUT:-30}"
LOG_DIR="${LOG_DIR:-$REPO_ROOT/.tmp/${E2E_LOG_NAME:-encrypttags-e2e}/$(date +%Y%m%d-%H%M%S)}"
CLEAN_REDIS_ON_EXIT="${CLEAN_REDIS_ON_EXIT:-0}"
NODE_BIN="${NODE_BIN:-$REPO_ROOT/build/hymx-node}"
DRY_RUN="${DRY_RUN:-0}"
NODE_PID="${NODE_PID:-}"
CURRENT_STEP="${CURRENT_STEP:-}"
E2E_LOG_PREFIX="${E2E_LOG_PREFIX:-encrypttags-e2e}"

log() {
	printf '[%s] %s\n' "$E2E_LOG_PREFIX" "$*"
}

step() {
	CURRENT_STEP="$*"
	log ""
	log "========== $CURRENT_STEP =========="
}

result() {
	log "RESULT: $*"
}

run() {
	log "+ $*"
	if [[ "$DRY_RUN" == "1" ]]; then
		return 0
	fi
	"$@"
}

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "missing required command: $1" >&2
		exit 127
	}
}

require_e2e_cmds() {
	require_cmd docker
	require_cmd go
	require_cmd grep
	require_cmd awk
}

cleanup_node() {
	if [[ -n "$NODE_PID" ]] && kill -0 "$NODE_PID" >/dev/null 2>&1; then
		log "stopping node pid=$NODE_PID"
		kill -INT "$NODE_PID" >/dev/null 2>&1 || true
		for _ in $(seq 1 "$NODE_STOP_TIMEOUT"); do
			if ! kill -0 "$NODE_PID" >/dev/null 2>&1; then
				wait "$NODE_PID" >/dev/null 2>&1 || true
				NODE_PID=""
				return 0
			fi
			sleep 1
		done
		log "node did not stop after SIGINT; sending SIGTERM"
		kill -TERM "$NODE_PID" >/dev/null 2>&1 || true
		wait "$NODE_PID" >/dev/null 2>&1 || true
		NODE_PID=""
	fi
}

cleanup() {
	if [[ -n "$CURRENT_STEP" ]]; then
		log "cleanup after: $CURRENT_STEP"
	fi
	cleanup_node
	if [[ "$CLEAN_REDIS_ON_EXIT" == "1" ]]; then
		docker rm -f "$REDIS_CONTAINER" >/dev/null 2>&1 || true
	fi
}

trap cleanup EXIT

wait_for_log() {
	local file="$1"
	local pattern="$2"
	local timeout="$3"
	local label="$4"

	for _ in $(seq 1 "$timeout"); do
		if [[ -f "$file" ]] && grep -Eq "$pattern" "$file"; then
			return 0
		fi
		if [[ -n "$NODE_PID" ]] && ! kill -0 "$NODE_PID" >/dev/null 2>&1; then
			echo "$label exited before readiness. Log: $file" >&2
			tail -n 80 "$file" >&2 || true
			return 1
		fi
		sleep 1
	done

	echo "timed out waiting for $label readiness pattern: $pattern" >&2
	echo "Log: $file" >&2
	tail -n 80 "$file" >&2 || true
	return 1
}

wait_for_redis() {
	for _ in $(seq 1 "$REDIS_READY_TIMEOUT"); do
		if docker exec "$REDIS_CONTAINER" redis-cli ping >/dev/null 2>&1; then
			log "redis ready: $REDIS_CONTAINER"
			return 0
		fi
		sleep 1
	done

	echo "timed out waiting for redis container: $REDIS_CONTAINER" >&2
	docker logs "$REDIS_CONTAINER" >&2 || true
	return 1
}

prepare_redis_and_build_node() {
	if [[ "$DRY_RUN" == "1" ]]; then
		log "+ docker rm -f $REDIS_CONTAINER"
		log "+ docker run -d --name $REDIS_CONTAINER -p 6379:6379 $REDIS_IMAGE"
		log "+ docker exec $REDIS_CONTAINER redis-cli ping"
		log "+ go build -o $NODE_BIN ./cmd"
		return 0
	fi

	(
		cd "$REPO_ROOT"
		run go build -o "$NODE_BIN" ./cmd
	)
	log "+ docker rm -f $REDIS_CONTAINER"
	docker rm -f "$REDIS_CONTAINER" >/dev/null 2>&1 || true
	run docker run -d --name "$REDIS_CONTAINER" -p 6379:6379 "$REDIS_IMAGE" >/dev/null
	wait_for_redis
}

start_node() {
	local phase="$1"
	local log_file="$LOG_DIR/node-$phase.log"

	log "starting node ($phase); log=$log_file"
	if [[ "$DRY_RUN" == "1" ]]; then
		log "+ (cd $REPO_ROOT/cmd && ../build/hymx-node >$log_file 2>&1 &)"
		return 0
	fi

	local old_pwd
	old_pwd="$(pwd -P)"
	cd "$REPO_ROOT/cmd"
	"$NODE_BIN" >"$log_file" 2>&1 &
	NODE_PID=$!
	cd "$old_pwd"

	wait_for_log "$log_file" "$NODE_READY_PATTERN" "$NODE_READY_TIMEOUT" "node ($phase)"
	result "node ready ($phase), pid=$NODE_PID, log=$log_file"
}

stop_node_for_checkpoint() {
	local log_file="$LOG_DIR/node-stop.log"

	cleanup_node
	if [[ "$DRY_RUN" == "1" ]]; then
		log "+ stop node with SIGINT"
		return 0
	fi

	sleep 1
	{
		find "$REPO_ROOT/ckp" -name 'ckp-*.json' -type f -print 2>/dev/null || true
		find "$REPO_ROOT/cmd/ckp" -name 'ckp-*.json' -type f -print 2>/dev/null || true
	} >"$log_file"
	result "node stopped; checkpoint file list recorded in $log_file"
}

run_examples() {
	local name="$1"
	shift
	local log_file="$LOG_DIR/examples-$name.log"

	log "running examples $name; log=$log_file"
	if [[ "$DRY_RUN" == "1" ]]; then
		log "+ (cd $REPO_ROOT && go run ./examples $*)"
		return 0
	fi

	(
		cd "$REPO_ROOT"
		go run ./examples "$@"
	) | tee "$log_file"
	result "examples $name completed, log=$log_file"
}

extract_process_id() {
	local log_file="$1"
	awk -F'pid=' '/^PROCESS pid=/{print $2; exit}' "$log_file"
}
