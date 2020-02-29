#!/bin/bash
set -e

while ! nc -z $SQL_HOSTNAME 3306;
  do
    echo sleeping;
    sleep 1;
  done;
  echo Connected!;

CMD="${@}"
if [ "${CMD}" = "" ]; then
        php /conspectus/bin/db.php && exec "apache2-foreground"
else
        exec "$@"
fi;
