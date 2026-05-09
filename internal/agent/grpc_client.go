package agent

import (
	"context"
	"fmt"

	pb "github.com/eugegm01-dev/metrics/api/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
}

func NewGRPCClient(addr string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}
	return &GRPCClient{
		conn:   conn,
		client: pb.NewMetricsClient(conn),
	}, nil
}

func (gc *GRPCClient) Close() error {
	return gc.conn.Close()
}

func (gc *GRPCClient) SendBatch(ctx context.Context, metrics []Metric) error {
	pbMetrics := make([]*pb.Metric, 0, len(metrics))
	for _, m := range metrics {
		mt := pb.Metric_GAUGE
		if m.MType == "counter" {
			mt = pb.Metric_COUNTER
		}

		// Opaque API builder – fields are direct values
		pm := (&pb.Metric_builder{
			Id:    m.ID,
			Type:  mt,
			Delta: m.Delta,
			Value: m.Value,
		}).Build()
		pbMetrics = append(pbMetrics, pm)
	}

	req := (&pb.UpdateMetricsRequest_builder{
		Metrics: pbMetrics,
	}).Build()

	// Attach local IP metadata if available
	localIP := getLocalIP()
	if localIP != "" {
		md := metadata.New(map[string]string{"x-real-ip": localIP})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	_, err := gc.client.UpdateMetrics(ctx, req)
	return err
}
