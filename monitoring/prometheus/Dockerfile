FROM prom/prometheus:v2.53.0

# USER root

COPY prometheus.yml /etc/prometheus/prometheus.yml

# # Create prometheus directory and set permissions
# RUN mkdir -p /prometheus && \
#     chown -R nobody:nobody /prometheus && \
#     chmod -R 777 /prometheus

# USER nobody

EXPOSE 9090

CMD ["--config.file=/etc/prometheus/prometheus.yml", \
     "--storage.tsdb.path=/prometheus", \
     "--web.enable-lifecycle", \
     "--storage.tsdb.retention.time=30d", \
     "--web.console.libraries=/usr/share/prometheus/console_libraries", \
     "--web.console.templates=/usr/share/prometheus/consoles"]