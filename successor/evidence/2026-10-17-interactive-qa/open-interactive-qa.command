#!/bin/zsh
set -eu
readonly evidence_dir="${0:A:h}"
readonly qa_app='/tmp/fort058-interactive-qa-20261017/Build/Products/Debug/FortQA.app'
readonly qa_session='75abd0d4-45b0-475b-a7a7-2eee514e6e38'
/usr/bin/shasum -a 256 -c "$evidence_dir/qa-app-sha256.txt" >/dev/null
/usr/bin/codesign --verify --deep --strict "$qa_app"
exec /usr/bin/open "$qa_app" --args --qa-session "$qa_session"
