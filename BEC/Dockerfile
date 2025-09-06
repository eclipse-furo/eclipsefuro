FROM golang:1.24-alpine

ENV SIMPLEGENEREATOR_VERSION=1.0.1
ENV FURO_VERSION=1.39.1
ENV GATEWAY_VERSION=2.27.1
ENV PROTOC_GEN_OPENMODELS_VERSION=1.43.2
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
ENV GOPRIVATE=github.com/theNorstroem

# Install ssh client and git
RUN apk add --no-cache bash openssh-client git curl wget ca-certificates openssh jq nodejs npm

# Download public key for github.com
RUN mkdir -p -m 0600 ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts
RUN ssh-keygen -q -t ed25519 -f /root/.ssh/id_ed25519 -N '""' -C '"furoBEC"'

# install protoc
RUN set -eux; \
    curl -L https://github.com/protocolbuffers/protobuf/releases/download/v$PROTOC_VERSION/protoc-$PROTOC_VERSION-linux-x86_64.zip -o /tmp/protoc.zip; \
    unzip /tmp/protoc.zip -d /usr/local; \
    # install glibc for alpine \
    wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub; \
    wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.35-r1/glibc-2.35-r1.apk; \
    apk add glibc-2.35-r1.apk; \
    rm -rf /tmp/*

# install the tools
RUN set -eux; \
     GO111MODULE=on go install \
     github.com/google/gnostic/cmd/protoc-gen-openapi@v$PROTOC_GEN_OPENAPI_VERSION

# install the tools
RUN set -eux; \
     GO111MODULE=on go install \
     github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models@v$PROTOC_GEN_OPENMODELS_VERSION


# install the tools
RUN set -eux; \
    GO111MODULE=on go install \
    github.com/eclipse/eclipsefuro/furo@v$FURO_VERSION \
    github.com/eclipse/eclipsefuro/furoc@v$FURO_VERSION \
    github.com/eclipse/eclipsefuro/furops@v$FURO_VERSION

RUN set -eux; \
    go install \
    github.com/eclipse/eclipsefuro/protoc-gen-furo-muspecs@v$FURO_VERSION \
    github.com/eclipse/eclipsefuro/protoc-gen-furo-specs@v$FURO_VERSION


RUN set -eux; \
    GO111MODULE=on go install \
    github.com/theNorstroem/simple-generator@v$SIMPLEGENEREATOR_VERSION


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