FROM ubuntu:xenial

# Building in the docker image increases the size because all the deps are installed
# if these are done in separate RUN commands they get put into diff. layers and removing later has no effect apparently
# as a result, all build/remove commands are pushed into one RUN command, the docker image goes from 808 MB to 300 by doing so
RUN apt-get -y update; \
    apt-get -y install software-properties-common dpkg-dev git; \
    add-apt-repository -y ppa:nginx/stable; \
    sed -i '/^#.* deb-src /s/^#//' /etc/apt/sources.list.d/nginx-ubuntu-stable-xenial.list; \
    apt-get -y update; \
    apt-get -y source nginx; \
    cd $(find . -maxdepth 1 -type d -name "nginx*") && \
    ls -ahl && \
    git clone https://github.com/arut/nginx-rtmp-module.git && \
    sed -i "s|common_configure_flags := \\\|common_configure_flags := \\\--add-module=$(cd  nginx-rtmp-module && pwd) \\\|" debian/rules && \
    cat debian/rules && echo "^^" && \
    apt-get -y build-dep nginx && \
    dpkg-buildpackage -b && \
    cd .. && ls -ahl && \
    dpkg --install $(find . -maxdepth 1 -type f -name "nginx-common*") && \
    dpkg --install $(find . -maxdepth 1 -type f -name "libnginx*") && \
    dpkg --install $(find . -maxdepth 1 -type f -name "nginx-full*"); \
    apt-get -y remove software-properties-common dpkg-dev git; \
    apt-get -y install aptitude; \
    aptitude -y markauto $(apt-cache showsrc nginx | sed -e '/Build-Depends/!d;s/Build-Depends: \|,\|([^)]*),*\|\[[^]]*\]//g'); \
    apt-get -y autoremove; \
    apt-get -y remove aptitude; \
    apt-get -y autoremove; \
    rm -rf ./*nginx*

# forward request and error logs to docker log collector
RUN ln -sf /dev/stdout /var/log/nginx/access.log
RUN ln -sf /dev/stderr /var/log/nginx/error.log

VOLUME ["/etc/nginx", "/var/cache/nginx"]

COPY nginx.conf /etc/nginx/nginx.conf

# prepare folder for volume with owner www-data
RUN mkdir /recordings
RUN chown -R www-data:www-data /recordings

CMD ["nginx", "-g", "daemon off;"]
