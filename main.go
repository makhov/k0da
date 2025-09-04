package main

import (
	"fmt"
	"os"

	"github.com/makhov/k0da/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

//podman run -it --rm \
//--name k0d-node \
//--hostname k0d-node \
//--privileged \
//--tmpfs /run \
//--tmpfs /var/run \
//--device /dev/null:/dev/kmsg \
//--security-opt unmask=ALL \
//--security-opt seccomp=unconfined \
//--security-opt apparmor=unconfined \
//--security-opt label=disable \
//-v k0d-var:/var \
//-v /lib/modules:/lib/modules:ro \
//-p 16443:6443 \
//quay.io/k0sproject/k0s:v1.33.3-k0s.0 k0s controller --enable-worker
