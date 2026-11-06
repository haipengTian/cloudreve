#!/bin/sh
set -eu
cd /cloudreve
# Keep the official helper process, but let Cloudreve receive SIGTERM directly.
if [ "${CR_ENABLE_ARIA2:-1}" = 1 ]; then
    supervisord -c ./aria2.supervisor.conf
fi
exec ./cloudreve "$@"
