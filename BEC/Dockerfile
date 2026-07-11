FROM golang:1.24-bookworm

ENV GATEWAY_VERSION=2.27.1
ENV PROTOC_GEN_FURO_VERSION=1.52.2
ENV PROTOC_GEN_OPENAPI_VERSION=0.7.0
ENV GEN_GO_VERSION=1.36.6
ENV GEN_GO_GRPC_VERSION=1.5.1
ENV YQ_VERSION=4.25.2
ENV BUF_VERSION=1.35.0
ENV PROTOC_VERSION=25.3
ENV GOBIN=$GOPATH/bin
ENV PATH="$PATH:$GOPATH/bin"
ENV PATH="/usr/local/sbin:$PATH"
ENV PS1="\e[0;34mフロー BEC \t# \e[m "

# Install ssh client, git, and common tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash openssh-client git curl wget ca-certificates jq nodejs npm unzip \
    && rm -rf /var/lib/apt/lists/*

# Download public key for github.com
RUN mkdir -p -m 0600 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts
RUN ssh-keygen -q -t ed25519 -f /root/.ssh/id_ed25519 -N '""' -C '"furoBEC"'

# install protoc
RUN set -eux; \
    curl -L https://github.com/protocolbuffers/protobuf/releases/download/v$PROTOC_VERSION/protoc-$PROTOC_VERSION-linux-x86_64.zip -o /tmp/protoc.zip; \
    unzip /tmp/protoc.zip -d /usr/local; \
    rm -rf /tmp/*

# install the tools
RUN set -eux; \
     GO111MODULE=on go install \
     github.com/google/gnostic/cmd/protoc-gen-openapi@v$PROTOC_GEN_OPENAPI_VERSION

# install furo protoc plugins
RUN set -eux; \
     GO111MODULE=on go install \
     github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models@v$PROTOC_GEN_FURO_VERSION \
     github.com/eclipse-furo/eclipsefuro/protoc-gen-om-jsonschema@v$PROTOC_GEN_FURO_VERSION \
     github.com/eclipse-furo/eclipsefuro/protoc-gen-debugfile@v$PROTOC_GEN_FURO_VERSION

# install grpc-gateway tools
RUN set -eux; \
    GO111MODULE=on go install \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v$GATEWAY_VERSION \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v$GATEWAY_VERSION

RUN set -eux; \
    GO111MODULE=on go install \
    google.golang.org/protobuf/cmd/protoc-gen-go@v$GEN_GO_VERSION

RUN set -eux; \
    GO111MODULE=on go install \
    google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$GEN_GO_GRPC_VERSION

RUN set -eux; \
    GO111MODULE=on go install github.com/mikefarah/yq/v4@v$YQ_VERSION; \
    rm -rf /go/pkg; \
    rm -rf /root/.cache/*

# install buf tools
RUN set -eux; \
    curl -sSL \
        "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(uname -s)-$(uname -m)" \
        -o "/usr/local/bin/buf" && \
    chmod +x "/usr/local/bin/buf"

WORKDIR /specs/
COPY docker-entrypoint.sh /usr/local/bin/
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["bash"]
