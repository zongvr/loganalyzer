#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
华傲数据 AI 开发考核 - JSONL 过程记录生成器

作用：
    读取你每轮填写的 rounds.json（指令 + commit 哈希 + 时间等），
    自动从 git 仓库抓取该 commit 的真实 diff，补全 modify_diff 字段，
    按考核规范输出标准 JSONL 文件（UTF-8、每行一条 JSON、无空行、无注释）。

    这样可避免手工复制 diff 出错/遗漏，且 diff 全部取自真实 git 历史，无法篡改。

用法：
    python3 tools/gen_jsonl.py \
        --repo . \
        --rounds rounds.json \
        --out "AI开发考核_姓名_loganalyzer.jsonl"

rounds.json 格式（数组，每个对象 = 一轮交互）：
[
  {
    "round_id": 1,
    "prompt_content": "你实际发给 Kilo Code 的完整自然语言指令",
    "commit_hash": "a1b2c3d4e5f6",        // 该轮对应的 git commit 短/长哈希
    "modify_time": "2026-08-08 10:20:00",  // 该轮交互时间 YYYY-MM-DD HH:MM:SS
    "agent_type": "Kilo Code",
    "dev_language": "Go"
    // modify_diff 可省略，脚本会从 git 自动抓取；若手动填了则以手动为准
  }
]

字段定义严格遵循考核说明第三部分。
"""
import argparse
import json
import subprocess
import sys


def get_diff(repo: str, commit: str) -> str:
    """优先取 parent..commit 的 diff；根提交则用 git show 兜底。"""
    try:
        out = subprocess.check_output(
            ["git", "-C", repo, "diff", f"{commit}^", commit],
            stderr=subprocess.DEVNULL,
        ).decode("utf-8")
        if out.strip():
            return out
    except subprocess.CalledProcessError:
        pass
    return subprocess.check_output(
        ["git", "-C", repo, "show", commit],
        stderr=subprocess.DEVNULL,
    ).decode("utf-8")


def main() -> None:
    ap = argparse.ArgumentParser(description="生成华傲数据考核标准 JSONL 过程记录")
    ap.add_argument("--repo", default=".", help="git 仓库路径，默认当前目录")
    ap.add_argument("--rounds", required=True, help="每轮记录的 JSON 文件")
    ap.add_argument("--out", required=True, help="输出 JSONL 文件名")
    args = ap.parse_args()

    with open(args.rounds, "r", encoding="utf-8") as f:
        rounds = json.load(f)

    required = ["round_id", "prompt_content", "commit_hash", "modify_time", "agent_type", "dev_language"]
    seen = set()
    lines = []
    for r in rounds:
        missing = [k for k in required if k not in r]
        if missing:
            sys.exit(f"[错误] round_id={r.get('round_id')} 缺少必填字段: {missing}")
        if r["round_id"] in seen:
            sys.exit(f"[错误] round_id={r['round_id']} 重复，必须唯一递增")
        seen.add(r["round_id"])

        diff = r.get("modify_diff") or get_diff(args.repo, r["commit_hash"])
        rec = {
            "round_id": r["round_id"],
            "prompt_content": r["prompt_content"],
            "modify_diff": diff,
            "commit_hash": r["commit_hash"],
            "modify_time": r["modify_time"],
            "agent_type": r["agent_type"],
            "dev_language": r["dev_language"],
        }
        lines.append(json.dumps(rec, ensure_ascii=False))

    with open(args.out, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")

    print(f"[完成] 已生成 {args.out}，共 {len(lines)} 轮记录。")


if __name__ == "__main__":
    main()
