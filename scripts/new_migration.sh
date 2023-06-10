#!/bin/bash
set -euo pipefail

if [ -z "${1+set}" ];  then
  echo "usage: ./scripts/new_migration.sh name_of_migration"
  exit 1
fi

DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
ROOT="$( dirname "$DIR" )"

CURTIME="$(date '+%Y%m%d%H%M%S')"

touch "$ROOT/db/sqldb/migrations/${CURTIME}_$1.up.sql"
touch "$ROOT/db/sqldb/migrations/${CURTIME}_$1.down.sql"
