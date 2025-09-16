set -euo pipefail

# 确认在 Git 仓库里
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  echo "❌ Not a git repository."; exit 1; }

# 如果有 origin，就先同步远端引用
if git remote get-url origin >/dev/null 2>&1; then
  git fetch origin --prune
fi

# 如果本地没有 main 分支，但远端有，就创建跟踪分支
if ! git show-ref --verify --quiet refs/heads/main; then
  if git show-ref --verify --quiet refs/remotes/origin/main; then
    git switch -c main --track origin/main
  else
    git switch -c main
  fi
else
  git switch main
fi

# 尝试仅快进更新（避免多余 merge 提交）
if git remote get-url origin >/dev/null 2>&1; then
  git pull --ff-only origin main || {
    echo "⚠️ 无法快进更新（可能本地 main 有额外提交）。请手动处理。"; exit 1; }
fi

echo "✅ local 'main' is up to date."
echo "ℹ️ 开新任务：  git switch -c feat/your-branch"
