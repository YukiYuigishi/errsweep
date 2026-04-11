#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BIN_PATH="${CACHE_BENCH_BIN:-$ROOT/sentinelfind}"
REPO_PATH="${CACHE_BENCH_REPO:-$ROOT/example}"
RUNS="${CACHE_BENCH_RUNS:-1}"
PRESET="${CACHE_BENCH_PRESET:-}"
MAX_AVG_REAL="${CACHE_BENCH_MAX_AVG_REAL:-}"
MAX_AVG_EXIT="${CACHE_BENCH_MAX_AVG_EXIT:-}"
FORMAT="${CACHE_BENCH_FORMAT:-plain}"

if [[ ! -x "$BIN_PATH" ]]; then
  echo "error: sentinelfind binary not found or not executable: $BIN_PATH" >&2
  echo "hint: run 'make build' first or set CACHE_BENCH_BIN" >&2
  exit 1
fi

if [[ ! -d "$REPO_PATH" ]]; then
  echo "error: benchmark repo not found: $REPO_PATH" >&2
  exit 1
fi

declare -a patterns=()
if [[ $# -gt 0 ]]; then
  patterns=("$@")
else
  case "$PRESET" in
    moby)
      patterns=("./..." "./daemon/..." "./api/..." "./pkg/...")
      ;;
    *)
      patterns=("./..." "./catalogservice/..." "./catalogrepo/...")
      ;;
  esac
fi

echo "cache-pattern benchmark"
echo "repo: $REPO_PATH"
echo "bin: $BIN_PATH"
echo "runs per pattern: $RUNS"
if [[ -n "$PRESET" ]]; then
  echo "preset: $PRESET"
fi
echo
if [[ "$FORMAT" == "markdown" ]]; then
  echo "| pattern | run | exit | real(s) | user(s) | sys(s) | output(bytes) |"
  echo "|---|---:|---:|---:|---:|---:|---:|"
else
  printf "%-22s %4s %4s %8s %8s %8s %14s\n" "pattern" "run" "exit" "real(s)" "user(s)" "sys(s)" "output(bytes)"
  printf "%-22s %4s %4s %8s %8s %8s %14s\n" "----------------------" "----" "----" "--------" "--------" "--------" "--------------"
fi

sum_real=0
sum_user=0
sum_sys=0
sum_bytes=0
sum_exit=0
sum_count=0

for pattern in "${patterns[@]}"; do
  for run_idx in $(seq 1 "$RUNS"); do
    time_log="$(mktemp)"
    out_json="$(mktemp)"
    (
      cd "$REPO_PATH"
      if /usr/bin/time -p "$BIN_PATH" -json "$pattern" >"$out_json" 2>"$time_log"; then
        echo 0 >"${time_log}.code"
      else
        echo $? >"${time_log}.code"
      fi
    )

    exit_code="$(cat "${time_log}.code")"
    real_s="$(awk '$1=="real"{print $2}' "$time_log")"
    user_s="$(awk '$1=="user"{print $2}' "$time_log")"
    sys_s="$(awk '$1=="sys"{print $2}' "$time_log")"
    out_bytes="$(wc -c < "$out_json" | tr -d ' ')"

    if [[ "$FORMAT" == "markdown" ]]; then
      echo "| \`$pattern\` | $run_idx | $exit_code | $real_s | $user_s | $sys_s | $out_bytes |"
    else
      printf "%-22s %4s %4s %8s %8s %8s %14s\n" "$pattern" "$run_idx" "$exit_code" "$real_s" "$user_s" "$sys_s" "$out_bytes"
    fi
    read sum_real sum_user sum_sys sum_bytes sum_exit sum_count < <(
      awk -v sr="$sum_real" -v su="$sum_user" -v ss="$sum_sys" -v sb="$sum_bytes" -v se="$sum_exit" -v sc="$sum_count" \
          -v r="$real_s" -v u="$user_s" -v s="$sys_s" -v b="$out_bytes" -v e="$exit_code" \
          'BEGIN { printf "%.6f %.6f %.6f %.6f %.6f %.0f\n", sr+r, su+u, ss+s, sb+b, se+e, sc+1 }'
    )

    rm -f "$time_log" "${time_log}.code" "$out_json"
  done
done

if [[ "$sum_count" -gt 0 ]]; then
  avg_real="$(awk -v v="$sum_real" -v c="$sum_count" 'BEGIN{printf "%.4f", v/c}')"
  avg_user="$(awk -v v="$sum_user" -v c="$sum_count" 'BEGIN{printf "%.4f", v/c}')"
  avg_sys="$(awk -v v="$sum_sys" -v c="$sum_count" 'BEGIN{printf "%.4f", v/c}')"
  avg_bytes="$(awk -v v="$sum_bytes" -v c="$sum_count" 'BEGIN{printf "%.0f", v/c}')"
  avg_exit="$(awk -v v="$sum_exit" -v c="$sum_count" 'BEGIN{printf "%.2f", v/c}')"
  echo
  if [[ "$FORMAT" == "markdown" ]]; then
    echo "| aggregate | count | avg exit | avg real(s) | avg user(s) | avg sys(s) | avg output(bytes) |"
    echo "|---|---:|---:|---:|---:|---:|---:|"
    echo "| all patterns | $sum_count | $avg_exit | $avg_real | $avg_user | $avg_sys | $avg_bytes |"
  else
    printf "%-22s %6s %10s %12s %12s %12s %18s\n" "aggregate" "count" "avg exit" "avg real(s)" "avg user(s)" "avg sys(s)" "avg output(bytes)"
    printf "%-22s %6s %10s %12s %12s %12s %18s\n" "----------------------" "------" "----------" "------------" "------------" "------------" "------------------"
    printf "%-22s %6s %10s %12s %12s %12s %18s\n" "all patterns" "$sum_count" "$avg_exit" "$avg_real" "$avg_user" "$avg_sys" "$avg_bytes"
  fi

  if [[ -n "$MAX_AVG_REAL" ]]; then
    if ! awk -v v="$avg_real" -v max="$MAX_AVG_REAL" 'BEGIN{exit !(v <= max)}'; then
      echo "error: avg real(s) threshold exceeded: $avg_real > $MAX_AVG_REAL" >&2
      exit 1
    fi
  fi
  if [[ -n "$MAX_AVG_EXIT" ]]; then
    if ! awk -v v="$avg_exit" -v max="$MAX_AVG_EXIT" 'BEGIN{exit !(v <= max)}'; then
      echo "error: avg exit threshold exceeded: $avg_exit > $MAX_AVG_EXIT" >&2
      exit 1
    fi
  fi
fi
