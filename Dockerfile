FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.22-openshift-4.17 AS builder
WORKDIR /go/src/github.com/openshift/cluster-bootstrap
COPY . .
ENV GO_PACKAGE github.com/openshift/cluster-bootstrap
RUN go build -ldflags "-X $GO_PACKAGE/pkg/version.Version=$(git describe --long --tags --abbrev=7 --match 'v[0-9]*')" ./cmd/cluster-bootstrap

FROM registry.ci.openshift.org/ocp/4.17:base-rhel9
COPY --from=builder /go/src/github.com/openshift/cluster-bootstrap/cluster-bootstrap /
ENTRYPOINT ["/cluster-bootstrap"]
COPY manifests /manifests
LABEL io.openshift.release.operator true
