package info

import (
	"context"

	"github.com/acorn-io/mink/pkg/types"
	apiv1 "github.com/acorn-io/runtime/pkg/apis/api.acorn.io/v1"
	"github.com/acorn-io/runtime/pkg/encryption"
	"github.com/acorn-io/runtime/pkg/encryption/nacl"
	"github.com/acorn-io/runtime/pkg/info"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewStrategy(c client.WithWatch) *Strategy {
	return &Strategy{
		client: c,
	}
}

type Strategy struct {
	client client.WithWatch
}

func (s *Strategy) NewList() types.ObjectList {
	return &apiv1.InfoList{}
}

func (s *Strategy) New() types.Object {
	return &apiv1.Info{}
}

func (s *Strategy) List(ctx context.Context, _ string, _ storage.ListOptions) (types.ObjectList, error) {
	var publicKeys []apiv1.EncryptionKey
	ns, _ := request.NamespaceFrom(ctx)
	if ns != "" {
		_, err := nacl.GetOrCreatePrimaryNaclKey(ctx, s.client, ns)
		if err != nil {
			return nil, err
		}
		publicKeys, err = encryption.GetEncryptionKeyList(ctx, s.client, ns)
		if err != nil {
			return nil, err
		}
	}

	i, err := info.Get(ctx, s.client)
	if err != nil {
		return nil, err
	}

	for key, regionInfo := range i.Regions {
		regionInfo.PublicKeys = publicKeys
		i.Regions[key] = regionInfo
	}

	return &apiv1.InfoList{
		Items: []apiv1.Info{
			*i,
		},
	}, nil
}
