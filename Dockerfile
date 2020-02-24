FROM shanemcc/docker-apache-php-base:latest
MAINTAINER Shane Mc Cormack <dataforce@dataforce.org.uk>

COPY . /conspectus

RUN \
  rm -Rfv /var/www/html && \
  chown -Rfv www-data: /conspectus/ /var/www/ && \
  ln -s /conspectus/public /var/www/html && \
  cd /conspectus/ && \
  mv config.php.example config.php && \
  su www-data --shell=/bin/bash -c "cd /conspectus; /usr/bin/composer install" && \
  echo "AliasMatch ^/resources/(.+?)/(.*) /conspectus/themes/\$1/resources/\$2" | tee /etc/apache2/conf-enabled/conspectus.conf && \
  echo "<Directory /conspectus/themes/>" | tee -a /etc/apache2/conf-enabled/conspectus.conf && \
  echo "  Require all granted" | tee -a /etc/apache2/conf-enabled/conspectus.conf && \
  echo "</Directory>" | tee -a /etc/apache2/conf-enabled/conspectus.conf

EXPOSE 80

ENTRYPOINT ["/conspectus/bin/dockerRun.sh"]
