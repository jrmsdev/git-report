FROM debian:forky-20251103-slim

LABEL maintainer="Jeremías Casteglione <jrmsdev@gmail.com>"
LABEL version="251116"

USER root:root
WORKDIR /root

ENV USER=root
ENV HOME=/root

ENV DEBIAN_FRONTEND=noninteractive

ENV APT_INSTALL='bash openssl ca-certificates media-types less wget golang python3 python3-venv git build-essential'

RUN apt-get clean \
	&& apt-get update -yy \
	&& apt-get dist-upgrade -yy --purge \
	&& apt-get install -yy --no-install-recommends ${APT_INSTALL} \
	&& apt-get clean \
	&& apt-get autoremove -yy --purge \
	&& rm -rf /var/lib/apt/lists/* \
		/var/cache/apt/archives/*.deb \
		/var/cache/apt/*cache.bin

RUN python3 -m venv /usr/local/venv

RUN /usr/local/venv/bin/pip install datasette
RUN ln -vsf /usr/local/venv/bin/datasette /usr/local/bin/datasette

ARG DEVEL_UID=1000
ARG DEVEL_GID=1000

ENV DEVEL_UID=${DEVEL_UID}
ENV DEVEL_GID=${DEVEL_GID}

RUN groupadd -o -g ${DEVEL_GID} devel \
	&& useradd -o -d /home/devel -m -c 'devel' -g ${DEVEL_GID} -u ${DEVEL_UID} devel \
	&& chmod -v 0750 /home/devel

RUN printf 'umask %s\n' '027' >>/home/devel/.profile
#RUN printf "export PS1='%s '\n" '\u@\h:\W\$' >>/home/devel/.profile

COPY ./docker/user-login.sh /usr/local/bin/user-login.sh
RUN chmod -v 0755 /usr/local/bin/user-login.sh

RUN install -v -m 0750 -o devel -g devel -d /opt/src

USER devel:devel
WORKDIR /home/devel

ENV USER=devel
ENV HOME=/home/devel

RUN go version
RUN python3 --version


RUN datasette --version

ENTRYPOINT ["/usr/local/bin/user-login.sh"]
