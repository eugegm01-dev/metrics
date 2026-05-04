package app

import (
	"context"
	"fmt"
	"net"
	"os"

	pb "github.com/eugegm01-dev/metrics/api/metrics/v1"
	"github.com/eugegm01-dev/metrics/internal/model"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type MetricsServer struct {
	pb.UnimplementedMetricsServer
	storage repository.Storage
	trusted string // CIDR доверенной подсети
	logger  *zap.Logger
}

func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	var modelMetrics []model.Metrics
	for _, m := range req.Metrics {
		metric := model.Metrics{
			ID:    m.GetId(),
			MType: m.GetType().String(), // "GAUGE" или "COUNTER"
		}
		switch m.GetType() {
		case pb.Metric_GAUGE:
			metric.Value = &m.Value
		case pb.Metric_COUNTER:
			metric.Delta = &m.Delta
		}
		modelMetrics = append(modelMetrics, metric)
	}

	if batchUpdater, ok := s.storage.(repository.BatchUpdater); ok {
		if err := batchUpdater.UpdateBatch(modelMetrics); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update metrics: %v", err)
		}
	} else {
		for _, m := range modelMetrics {
			switch m.MType {
			case model.Gauge:
				s.storage.UpdateGauge(m.ID, *m.Value)
			case model.Counter:
				s.storage.UpdateCounter(m.ID, *m.Delta)
			}
		}
	}
	return &pb.UpdateMetricsResponse{}, nil
}

// UnaryInterceptor – проверка доверенной подсети
func TrustedSubnetInterceptor(trustedCIDR string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if trustedCIDR == "" {
			return handler(ctx, req)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}
		ipValues := md.Get("x-real-ip")
		if len(ipValues) == 0 {
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip")
		}
		ip := net.ParseIP(ipValues[0])
		if ip == nil {
			return nil, status.Error(codes.PermissionDenied, "invalid ip")
		}
		_, subnet, err := net.ParseCIDR(trustedCIDR)
		if err != nil {
			return nil, status.Error(codes.Internal, "invalid trusted subnet")
		}
		if !subnet.Contains(ip) {
			return nil, status.Error(codes.PermissionDenied, "ip not in trusted subnet")
		}
		return handler(ctx, req)
	}
}

func StartGRPCServer(address string, storage repository.Storage, trustedCIDR string, logger *zap.Logger) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}
	s := grpc.NewServer(
		grpc.UnaryInterceptor(TrustedSubnetInterceptor(trustedCIDR)),
	)
	pb.RegisterMetricsServer(s, &MetricsServer{
		storage: storage,
		trusted: trustedCIDR,
		logger:  logger,
	})
	logger.Info("gRPC server listening", zap.String("addr", address))
	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Fatal("gRPC serve failed", zap.Error(err))
			os.Exit(1)
		}
	}()
	return nil
}
