## Build Release Images

Build release binaries + images

```
BUILD_IMAGE=bootkube PUSH_IMAGE=true ./build/build-image.sh
BUILD_IMAGE=checkpoint PUSH_IMAGE=true ./build/build-image.sh
```

## Updating checkpoint container

The hyperkube release will use a specific git hash for the checkpoint pod.
This requires a two step process to update the checkpoint container.

First update & commit changes to checkpoint code and build an image:

```
BUILD_IMAGE=checkpoint PUSH_IMAGE=true ./build/build-image.sh
```

Now that we have a checkpoint image released, we can reference that git hash in the api-server manifest:

```
# Edit api-server manifest
vi pkg/asset/internal/templates.go
# commmit / push changes

# Now build bootkube image with the updated manifest
BUILD_IMAGE=bootkube PUSH_IMAGE=true ./build/build-image.sh
```
