FROM registry.svc.ci.openshift.org/openshift/release:golang-1.10 AS builder
WORKDIR /go/src/github.com/openshift/cluster-bootstrap
COPY . .
ENV GO_PACKAGE github.com/openshift/cluster-bootstrap
RUN go build -ldflags "-X $GO_PACKAGE/pkg/version.Version=$(git describe --long --tags --abbrev=7 --match 'v[0-9]*')" ./cmd/cluster-bootstrap

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/cluster-bootstrap/cluster-bootstrap /
ENTRYPOINT ["/cluster-bootstrap"]
COPY manifests /manifests
COPY scripts/bootkube.sh /
LABEL io.openshift.release.operator true
