FROM redis:alpine

EXPOSE 26379

COPY sentinel.conf /etc/redis/sentinel.conf

CMD ["redis-sentinel", "/etc/redis/sentinel.conf"]
