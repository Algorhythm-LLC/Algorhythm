#!/usr/bin/env bash
# Инициализация submodules для сервисов Algorhythm
# Использование:
#   ./scripts/bootstrap-submodules.sh              # init + update существующих
#   ./scripts/bootstrap-submodules.sh --init-only   # только init (для пустых)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVICES_DIR="$REPO_ROOT/services"

cd "$REPO_ROOT"

# Если submodules ещё не добавлены — создаём заглушки
# После создания удалённых репозиториев: git submodule add <url> services/<name>
if [ ! -f .gitmodules ]; then
  echo "No .gitmodules found. Create service repos and add them:"
  echo "  git submodule add <control-plane-repo-url> services/control-plane"
  echo "  git submodule add <market-data-ingestor-repo-url> services/market-data-ingestor"
  echo "  # ... etc"
  exit 0
fi

git submodule init
git submodule update --recursive
echo "Submodules initialized."
