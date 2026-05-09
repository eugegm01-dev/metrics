package app

import (
	"context"
	"fmt"
	"net"

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
	logger  *zap.Logger
}

func (s *MetricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	var modelMetrics []model.Metrics
	for _, m := range req.GetMetrics() {
		metric := model.Metrics{
			ID:    m.GetId(),
			MType: m.GetType().String(),
		}
		switch m.GetType() {
		case pb.Metric_GAUGE:
			v := m.GetValue()
			metric.Value = &v
		case pb.Metric_COUNTER:
			d := m.GetDelta()
			metric.Delta = &d
		}
		modelMetrics = append(modelMetrics, metric)
	}

	if batchUpdater, ok := s.storage.(repository.BatchUpdater); ok {
		if err := batchUpdater.UpdateBatch(modelMetrics); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update metrics")
		}
	} else {
		for _, m := range modelMetrics {
			switch m.MType {
			case model.Gauge:
				_ = s.storage.UpdateGauge(m.ID, *m.Value)
			case model.Counter:
				_ = s.storage.UpdateCounter(m.ID, *m.Delta)
			}
		}
	}
	return &pb.UpdateMetricsResponse{}, nil
}

func TrustedSubnetInterceptor(subnet *net.IPNet) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}
		ipValues := md.Get("x-real-ip")
		if len(ipValues) == 0 {
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip")
		}
		ip := net.ParseIP(ipValues[0])
		if ip == nil || !subnet.Contains(ip) {
			zap.L().Warn("gRPC request from untrusted IP", zap.String("ip", ipValues[0]))
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		}
		return handler(ctx, req)
	}
}

func StartGRPCServer(address string, storage repository.Storage, trustedCIDR string, logger *zap.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("gRPC listen: %w", err)
	}

	var opts []grpc.ServerOption
	if trustedCIDR != "" {
		_, parsedSubnet, err := net.ParseCIDR(trustedCIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted subnet: %w", err)
		}
		opts = append(opts, grpc.UnaryInterceptor(TrustedSubnetInterceptor(parsedSubnet)))
	}

	s := grpc.NewServer(opts...)
	pb.RegisterMetricsServer(s, &MetricsServer{
		storage: storage,
		logger:  logger,
	})
	logger.Info("gRPC server listening", zap.String("addr", address))

	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error("gRPC serve failed", zap.Error(err))
		}
	}()
	return s, nil
}
