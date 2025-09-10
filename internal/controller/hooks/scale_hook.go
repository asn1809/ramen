// SPDX-FileCopyrightText: The RamenDR authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"github.com/go-logr/logr"
	"github.com/ramendr/ramen/internal/controller/kubeobjects"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ScaleHook struct {
	Reader client.Reader
	Hook   *kubeobjects.HookSpec
}

func (s ScaleHook) Execute(log logr.Logger) error {
	// TODO: Scale hook implementation needs to be done here.

	op := s.Hook.ScaleOp
	return nil
}
