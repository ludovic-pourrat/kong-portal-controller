package main

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"kong-portal-controller/internal/cmd/rootcmd"
)

func main() {
	rootcmd.Execute(ctrl.SetupSignalHandler())
}
