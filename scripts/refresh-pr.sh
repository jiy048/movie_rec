set -euo pipefail

# 确认在 Git 仓库里
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  echo "❌ Not a git repository."; exit 1; }

current_branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$current_branch" == "main" ]]; then
  echo "❌ 当前在 main。请切到功能分支后再运行。"
  exit 1
fi

# 必须有 origin/main 作为基线
git remote get-url origin >/dev/null 2>&1 || {
  echo "❌ 未配置 origin 远端。"; exit 1; }

git fetch origin --prune

echo "♻️  Rebase $current_branch onto origin/main ..."
if git rebase origin/main; then
  echo "🔐 推送（--force-with-lease 更安全）..."
  # 如果没有上游追踪，就先建追踪
  if git rev-parse --abbrev-ref --symbolic-full-name @{u} >/dev/null 2>&1; then
    git push --force-with-lease
  else
    git push --force-with-lease -u origin "$current_branch"
  fi
  echo "✅ 已基于最新主干并更新远端分支。"
else
  echo "⚠️ rebase 失败（可能产生冲突）。请："
  echo "   1) 解决冲突并 git add <文件>"
  echo "   2) git rebase --continue  (或 --abort 放弃)"
  exit 1
fi
