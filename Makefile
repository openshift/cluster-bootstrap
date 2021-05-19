all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/deps-gomod.mk \
	targets/openshift/images.mk \
)

$(call build-image,origin-$(GO_PACKAGE),./Dockerfile,.)
# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build
$(call build-image,ocp-cluster-bootstrap,$(IMAGE_REGISTRY)/ocp/4.2:cluster-bootstrap,./Dockerfile.rhel7,.)

$(call verify-golang-versions,Dockerfile)

clean:
	$(RM) ./cluster-bootstrap
.PHONY: clean

test-e2e: # there is none right now
