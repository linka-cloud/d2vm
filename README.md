# d2vm

*Build virtual machine image from Docker images*

**Status**: *alpha*

## Supported Environments:

**Only Linux is supported.**

If you want to run it on OSX or Windows (untested) you can use Docker for it:

```bash
docker run --rm -i -t \
    --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $(pwd):/build \
    -w /build \
    linkacloud/d2vm bash
```
