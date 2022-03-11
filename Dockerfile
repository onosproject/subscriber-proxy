FROM onosproject/golang-build:v0.6.10 as build

ARG LOCAL_AETHER_MODELS
ARG org_label_schema_version=unknown
ARG org_label_schema_vcs_url=unknown
ARG org_label_schema_vcs_ref=unknown
ARG org_label_schema_build_date=unknown
ARG org_opencord_vcs_commit_date=unknown
ARG org_opencord_vcs_dirty=unknown

ENV ADAPTER_ROOT=$GOPATH/src/github.com/onosproject/subscriber-proxy
ENV CGO_ENABLED=0

RUN mkdir -p $ADAPTER_ROOT/

COPY . $ADAPTER_ROOT/

# If LOCAL_AETHER_MODELS was used, then patch the go.mod file to load
# the models from the local source.
RUN if [ -n "$LOCAL_AETHER_MODELS" ] ; then \
    echo "replace github.com/onosproject/config-models/modelplugin/aether-3.0.0 => ./local-aether-models/aether-3.0.0" >> $ADAPTER_ROOT/go.mod; \
    echo "replace github.com/onosproject/config-models/modelplugin/aether-4.0.0 => ./local-aether-models/aether-4.0.0" >> $ADAPTER_ROOT/go.mod; \
    fi

RUN cat $ADAPTER_ROOT/go.mod


RUN cd $ADAPTER_ROOT && GO111MODULE=on go build -o /go/bin/subscriber-proxy \
        -ldflags \
        "-X github.com/onosproject/subscriber-proxy/internal/pkg/version.Version=$org_label_schema_version \
         -X github.com/onosproject/subscriber-proxy/internal/pkg/version.GitCommit=$org_label_schema_vcs_ref  \
         -X github.com/onosproject/subscriber-proxy/internal/pkg/version.GitDirty=$org_opencord_vcs_dirty \
         -X github.com/onosproject/subscriber-proxy/internal/pkg/version.GoVersion=$(go version 2>&1 | sed -E  's/.*go([0-9]+\.[0-9]+\.[0-9]+).*/\1/g') \
         -X github.com/onosproject/subscriber-proxy/internal/pkg/version.Os=$(go env GOHOSTOS) \
         -X github.com/onosproject/subscriber-proxy/internal/pkg/version.Arch=$(go env GOHOSTARCH) \
         -X github.com/onosproject/subscriber-proxy/internal/pkg/version.BuildTime=$org_label_schema_build_date" \
         ./cmd/subscriber-proxy

FROM alpine:3.11
RUN apk add bash openssl curl libc6-compat

ENV HOME=/home/subscriber-proxy

RUN mkdir $HOME
WORKDIR $HOME

COPY --from=build /go/bin/subscriber-proxy /usr/local/bin/

