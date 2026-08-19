#!/usr/bin/env bash
# Extracts YouTube/Google cookies straight from Zen's local cookies.sqlite
# (Firefox-format, unencrypted) via yt-dlp's own --cookies-from-browser —
# no browser extension involved. yt-dlp copies the DB internally before
# reading, so it's safe to run while Zen is open.
set -euo pipefail

REMOTE_HOST="contabo_vps"
REMOTE_PATH="/opt/workhub/cookies.txt"
# mktemp -u: just generate a name, don't create the file — yt-dlp's
# Netscape-cookie-file parser errors on a pre-existing empty file
# ("does not look like a Netscape format cookies file"), confirmed
# manually. It creates the file itself once it has real content to write.
COOKIES_FILE="$(mktemp -u)"
# Any stable, always-public video — this is just used to trigger yt-dlp's
# extractor pipeline (which is what actually pulls/refreshes the cookies);
# --skip-download means the audio itself never gets fetched.
TRIGGER_URL="https://www.youtube.com/watch?v=dQw4w9WgXcQ"

cleanup() { rm -f "$COOKIES_FILE"; }
trap cleanup EXIT

# Pick whichever Zen profile actually has a cookies.sqlite, most recently
# modified if there's more than one — avoids hardcoding a profile name that
# breaks the moment a new profile gets created.
PROFILE_DIR="$(find "$HOME/.zen" -maxdepth 1 -mindepth 1 -type d -exec test -e '{}/cookies.sqlite' \; -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)"

if [ -z "$PROFILE_DIR" ]; then
  echo "No Zen profile with cookies.sqlite found under ~/.zen" >&2
  exit 1
fi

# The trigger video can fail format-selection (YouTube client-fallback
# quirks, unrelated to cookie extraction — confirmed manually: yt-dlp logs
# "Extracted N cookies from firefox" and writes $COOKIES_FILE *before* it
# ever gets to picking a format) — `|| true` so that doesn't abort the
# whole script under `set -e`. The real pass/fail signal is the file-size
# check right below, not this command's exit code.
yt-dlp \
  --cookies-from-browser "firefox:$PROFILE_DIR" \
  --cookies "$COOKIES_FILE" \
  --skip-download \
  --no-warnings \
  "$TRIGGER_URL" || true

if [ ! -s "$COOKIES_FILE" ]; then
  echo "cookies.txt came out empty — aborting, not overwriting the server's copy" >&2
  exit 1
fi

# Not `scp` straight to the destination path: scp (SFTP-mode, the OpenSSH
# default) uploads to a temp file and renames it over the target — that
# swaps the file's inode. /opt/workhub/cookies.txt is bind-mounted into
# the backend's Docker container, and a bind mount keeps pointing at the
# old inode after a rename-replace until the container restarts (confirmed
# by hand: freshly-exported cookies still failed until a restart, and the
# *old* file worked again post-restart — so it was never about cookie
# content, purely this). Writing through `cat >` on the remote instead
# truncates and rewrites the *same* inode in place, so the bind mount
# stays valid and no restart is ever needed.
#
# BatchMode=yes on both hops: fail fast instead of hanging on an
# interactive password prompt if key auth is ever misconfigured — never
# silently blocks waiting for input in an unattended run.
REMOTE_TMP="/tmp/cookies-incoming-$$.txt"
scp -o BatchMode=yes "$COOKIES_FILE" "$REMOTE_HOST:$REMOTE_TMP"
ssh -o BatchMode=yes "$REMOTE_HOST" "cat '$REMOTE_TMP' > '$REMOTE_PATH' && rm -f '$REMOTE_TMP'"

echo "cookies synced: $(date -Iseconds)"
