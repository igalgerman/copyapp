#!/usr/bin/env bash
set -euo pipefail

# Owner-run only. keytool prompts securely; no password appears in shell args,
# files or output. This helper never uploads the key or changes GitHub secrets.
if [[ $# -ne 1 ]]; then
  echo 'Usage: bash android/create-signing-key.sh /private/backup/copyapp-release.p12' >&2
  exit 2
fi
project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
key_path=$(python3 - "$1" "$project_root" <<'CHECK'
import pathlib, sys
key = pathlib.Path(sys.argv[1]).expanduser().resolve()
repo = pathlib.Path(sys.argv[2]).resolve()
if key == repo or repo in key.parents:
    raise SystemExit('Choose a private signing-key path outside the repository.')
print(key)
CHECK
)
if [[ -e "$key_path" ]]; then
  echo 'Refusing to overwrite an existing signing key.' >&2
  exit 1
fi
umask 077
keytool -genkeypair -keystore "$key_path" -storetype PKCS12 \
  -alias copyapp-release -keyalg RSA -keysize 3072 -validity 10000 \
  -dname 'CN=CopyApp Android Release'
echo 'Public certificate information (record the SHA-256 fingerprint):'
keytool -list -v -keystore "$key_path" -alias copyapp-release
echo 'Back up this keystore and password privately. See docs/ANDROID.md for GitHub secret setup.'
