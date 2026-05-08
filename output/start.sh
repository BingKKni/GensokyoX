#!/usr/bin/env bash
RESTART_DELAY=2         # 异常退出后等待多少秒再重启 (避免死循环)
MAX_RESTARTS=0          # 0=无限重启, >0=达到该次数后放弃
KEEP_LAST_LINES=1000    # 崩溃日志只保留输出末尾多少行 (含 panic 栈)

restart_count=0

set -uo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$SCRIPT_DIR"

BIN="./gensokyo10" #这里后面编号代表第X次编译，每次编译后都要重命名给后面加一个数字编号，方便启动失败时回退到上一个编号的版本

if [[ ! -f "$BIN" ]]; then
  echo "Error: $BIN not found. Put this script next to gensokyox." >&2
  exit 1
fi

chmod +x "$BIN" 2>/dev/null || true

LOG_DIR="./log"
mkdir -p "$LOG_DIR"

trap 'echo ""; echo "[start.sh] 收到退出信号"; exit 0' INT TERM

while true; do
    restart_count=$((restart_count + 1))
    if [[ $restart_count -gt 1 ]]; then
        echo "[start.sh] 第 $restart_count 次启动 $BIN"
    fi

    # 临时文件接住本次运行的输出 (终端仍可见, 正常退出会删掉)
    tmp_log="$(mktemp "$LOG_DIR/.running.XXXXXX")"

    "$BIN" "$@" > >(tee -a "$tmp_log") 2> >(tee -a "$tmp_log" >&2)
    exit_code=$?

    # 等 tee 子进程刷盘
    sleep 0.2

    case $exit_code in
        0)
            rm -f "$tmp_log"
            echo "[start.sh] 框架正常退出"
            exit 0
            ;;
        130|143)
            # 130 = SIGINT (Ctrl+C), 143 = SIGTERM
            rm -f "$tmp_log"
            echo "[start.sh] 框架被信号终止 (exit=$exit_code)"
            exit "$exit_code"
            ;;
    esac

    # 异常退出: 把输出尾部存为 crash 日志
    crash_log="$LOG_DIR/crash-$(date +%Y-%m-%d_%H-%M-%S).log"
    {
        echo "==== Gensokyo 异常退出 ===="
        echo "时间    : $(date '+%Y-%m-%d %H:%M:%S')"
        echo "退出码  : $exit_code"
        echo "末尾 $KEEP_LAST_LINES 行输出 (含 panic 栈):"
        echo "==========================="
        tail -n "$KEEP_LAST_LINES" "$tmp_log"
    } > "$crash_log"
    rm -f "$tmp_log"

    echo ""
    echo "[start.sh] !!! 框架异常退出 (exit=$exit_code)"
    echo "[start.sh] !!! 崩溃日志: $crash_log"

    if [[ $MAX_RESTARTS -gt 0 && $restart_count -ge $MAX_RESTARTS ]]; then
        echo "[start.sh] 达到最大重启次数 $MAX_RESTARTS, 放弃"
        exit 1
    fi

    echo "[start.sh] $RESTART_DELAY 秒后自动重启..."
    sleep "$RESTART_DELAY"
done
