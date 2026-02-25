package attestation

import (
	"context"
	"fmt"

	dstacksdk "github.com/Dstack-TEE/dstack/sdk/go/dstack"
)

// DstackInfoCollector collects local attestation material from dstack guest-agent Info().
type DstackInfoCollector struct {
	client *dstacksdk.DstackClient
}

func NewDstackInfoCollector(endpoint string) *DstackInfoCollector {
	opts := []dstacksdk.DstackClientOption{}
	if endpoint != "" {
		opts = append(opts, dstacksdk.WithEndpoint(endpoint))
	}
	return &DstackInfoCollector{client: dstacksdk.NewDstackClient(opts...)}
}

func (c *DstackInfoCollector) Collect(ctx context.Context) (Bundle, error) {
	info, err := c.client.Info(ctx)
	if err != nil {
		return Bundle{}, fmt.Errorf("dstack info: %w", err)
	}
	return Bundle{
		AppCert:  info.AppCert,
		TCBInfo:  info.TcbInfo,
		AppID:    info.AppID,
		Instance: info.InstanceID,
		DeviceID: info.DeviceID,
	}, nil
}
