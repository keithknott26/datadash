#!/usr/bin/env -S docker build --compress -t keithknott26/datadash -f

FROM debian as build

RUN apt update
RUN apt install -y gcc git curl

RUN curl -skL https://dl.google.com/go/go1.13.linux-amd64.tar.gz \
	| tar --strip-components=0 -xzC /usr/local

ENV PATH "$PATH:/usr/local/go/bin"

WORKDIR /root/go/src/github.com/keithknott26/datadash
COPY ./ ./
RUN echo get build test install | xargs -n1 | xargs -n1 -I% -- go % .
RUN echo     build      install | xargs -n1 | xargs -n1 -I% -- go % ./cmd/datadash.go

FROM debian
WORKDIR /data
COPY --from=build /root/go/bin/datadash /usr/local/bin/datadash
ENTRYPOINT [ "/usr/local/bin/datadash" ]
CMD        [ ]
