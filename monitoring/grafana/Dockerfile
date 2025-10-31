FROM grafana/grafana:11.2.0

COPY provisioning/datasources /etc/grafana/provisioning/datasources
COPY provisioning/dashboards /etc/grafana/provisioning/dashboards
COPY dashboards /etc/grafana/dashboards

ENV GF_SECURITY_ADMIN_USER=admin \
    GF_SECURITY_ADMIN_PASSWORD=admin \
    GF_USERS_DEFAULT_THEME=dark \
    GF_SERVER_DOMAIN=peerpr-publi-hcouztyo5btp-1729772274.ap-southeast-1.elb.amazonaws.com \
    GF_SERVER_ROOT_URL=http://peerpr-publi-hcouztyo5btp-1729772274.ap-southeast-1.elb.amazonaws.com/grafana/ \
    GF_SERVER_SERVE_FROM_SUB_PATH=true

EXPOSE 3000