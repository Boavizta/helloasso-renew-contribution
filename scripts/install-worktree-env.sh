#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "Usage: sh scripts/install-worktree-env.sh /absolute/path/to/reference/.env" >&2
  exit 1
fi
case "$1" in
  /*) ;;
  *) echo "The reference .env path must be absolute." >&2; exit 1 ;;
esac
if [ ! -f "$1" ]; then
  echo "The reference .env must exist." >&2
  exit 1
fi
if git config --get core.hooksPath >/dev/null; then
  echo "Custom core.hooksPath detected; integrate scripts/hooks/post-checkout manually." >&2
  exit 1
fi

ROOT="$(git rev-parse --show-toplevel)"
COMMON="$(git rev-parse --git-common-dir)"
HOOK="$COMMON/hooks/post-checkout"
if [ -e "$HOOK" ] || [ -L "$HOOK" ]; then
  if ! grep -Fqx '# Share local secrets across worktrees without overwriting existing settings.' "$HOOK"; then
    echo "Existing post-checkout hook found; integrate scripts/hooks/post-checkout manually." >&2
    exit 1
  fi
fi

mkdir -p "$COMMON/hooks"
cp "$ROOT/scripts/hooks/post-checkout" "$HOOK"
chmod +x "$HOOK"
git config --local worktreeEnv.source "$1"
"$HOOK"
echo "Installed shared worktree .env hook. Existing .env files are preserved."
