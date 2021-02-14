FROM golang:1.9

WORKDIR /home/isucon/isubata/webapp
COPY . .

WORKDIR /home/isucon/isubata/webapp/go
RUN make

EXPOSE 5000

ENTRYPOINT ["/home/isucon/isubata/webapp/go/isubata"]
