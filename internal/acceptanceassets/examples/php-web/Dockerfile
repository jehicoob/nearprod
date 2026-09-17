FROM php:8.3-apache
# Example runtime. Pin a reviewed base digest in your own release pipeline.
RUN printf 'opcache.enable=1\nopcache.validate_timestamps=0\n' > /usr/local/etc/php/conf.d/nearprod.ini
COPY src/ /var/www/html/
COPY worker/ /opt/worker/
EXPOSE 80
