package main

import (
	"fmt"

	"github.com/openshift/build-machinery-go/make/examples/multiple-binaries/pkg/version"
)

func main() {
	fmt.Print(version.String())
}
