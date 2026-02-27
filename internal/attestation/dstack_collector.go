package attestation

import (
	"context"
	"fmt"

	dstacksdk "github.com/Dstack-TEE/dstack/sdk/go/dstack"
	"github.com/aspect-build/jingui/internal/logx"
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
	logx.Debugf("ratls.collect app_id=%q instance_id=%q device_id=%q app_cert_len=%d tcb_info_len=%d", info.AppID, info.InstanceID, info.DeviceID, len(info.AppCert), len(info.TcbInfo))
	return Bundle{
		AppCert:  info.AppCert,
		TCBInfo:  info.TcbInfo,
		AppID:    info.AppID,
		Instance: info.InstanceID,
		DeviceID: info.DeviceID,
	}, nil
}
