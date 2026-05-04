package agent

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/eugegm01-dev/metrics/api/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
	mu     sync.Mutex
}

func NewGRPCClient(addr string) (*GRPCClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	pbMetrics := make([]*pb.Metric, len(metrics))
	for i, m := range metrics {
		mt := pb.Metric_GAUGE
		if m.MType == "counter" {
			mt = pb.Metric_COUNTER
		}
		pm := &pb.Metric{
			Id:    m.ID,
			Type:  mt,
			Delta: m.Delta,
			Value: m.Value,
		}
		pbMetrics[i] = pm
	}
	req := &pb.UpdateMetricsRequest{Metrics: pbMetrics}

	// Добавляем метаданные с IP агентом
	localIP := getLocalIP() // уже есть в sender.go
	if localIP != "" {
		md := metadata.New(map[string]string{"x-real-ip": localIP})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	_, err := gc.client.UpdateMetrics(ctx, req)
	return err
}
