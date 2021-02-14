FROM golang:1.9

# ARG GIT_URL=https://github.com/isucon/isucon7-qualify.git

# RUN git clone $GIT_URL /home/isucon/isubata
WORKDIR /home/isucon/isubata/bench
COPY . .
RUN \
    go get github.com/constabulary/gb/... && \
    gb vendor restore && \
    make

ENTRYPOINT ["bin/bench"]
CMD ["-remotes", "web"]
