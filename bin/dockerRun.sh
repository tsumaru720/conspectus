#!/bin/bash
set -e

CMD="${@}"
if [ "${CMD}" = "" ]; then
        php /conspectus/bin/db.php && exec "apache2-foreground"
else
        exec "$@"
fi;
