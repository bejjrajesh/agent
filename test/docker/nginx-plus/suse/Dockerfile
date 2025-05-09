ARG OS_RELEASE
ARG OS_VERSION
ARG CONTAINER_REGISTRY

FROM ${CONTAINER_REGISTRY}/${OS_RELEASE}/${OS_VERSION} as install
LABEL maintainer="NGINX Docker Maintainers <docker-maint@nginx.com>"

ARG OS_VERSION
ARG PACKAGES_REPO
ARG NGINX_AGENT_VERSION

WORKDIR /agent
COPY ./test/docker/entrypoint.sh /agent/entrypoint.sh
COPY ./nginx-agent.conf /agent/nginx-agent.conf

RUN --mount=type=secret,id=nginx-crt,dst=/nginx-repo.crt \
    --mount=type=secret,id=nginx-key,dst=/nginx-repo.key \
    set -x \
    && mkdir -p /etc/ssl/nginx \
    && cat /nginx-repo.crt > /etc/ssl/nginx/nginx-repo.crt \
    && cat /nginx-repo.key > /etc/ssl/nginx/nginx-repo.key \
    && cat /etc/ssl/nginx/nginx-repo.crt /etc/ssl/nginx/nginx-repo.key > /etc/ssl/nginx/nginx-repo-bundle.crt \
    && zypper install ca-certificates \
    && nginxPackages=" \
        nginx-plus \
       " \
    &&  zypper addrepo -G -c "https://$PACKAGES_REPO/plus/sles/15?ssl_clientcert=/etc/ssl/nginx/nginx-repo-bundle.crt&ssl_verify=peer" nginx-plus \
    &&  zypper addrepo -G -c "https://$PACKAGES_REPO/nginx-agent/sles/15?ssl_clientcert=/etc/ssl/nginx/nginx-repo-bundle.crt&ssl_verify=peer" nginx-agent; \
    && zypper refresh \
    && zypper install -y $nginxPackages 

RUN if [ -z "$NGINX_AGENT_VERSION" ]; then zypper install -y nginx-agent; else zypper install -y nginx-agent=${NGINX_AGENT_VERSION}; fi
RUN rm -rf /etc/ssl/nginx/nginx-repo-bundle.crt /etc/ssl/nginx

FROM install as runtime

COPY --from=install /agent/entrypoint.sh /agent/entrypoint.sh
COPY --from=install /agent/nginx-agent.conf /etc/nginx-agent/nginx-agent.conf

RUN chmod +x /agent/entrypoint.sh
STOPSIGNAL SIGTERM
EXPOSE 80 443

ENTRYPOINT ["/agent/entrypoint.sh"]
