package grpc

import (
	"fmt"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/linkmeAman/universal-middleware/pkg/metrics"
)

// Server represents a gRPC server instance
type Server struct {
	server  *grpc.Server
	logger  *zap.Logger
	metrics *metrics.Metrics
	port    int
}

// NewServer creates a new gRPC server with middleware
func NewServer(logger *zap.Logger, m *metrics.Metrics, port int) *Server {
	// Create gRPC server with middleware chain
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_prometheus.UnaryServerInterceptor,
			grpc_zap.UnaryServerInterceptor(logger),
			grpc_recovery.UnaryServerInterceptor(),
		)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_prometheus.StreamServerInterceptor,
			grpc_zap.StreamServerInterceptor(logger),
			grpc_recovery.StreamServerInterceptor(),
		)),
	)

	// Initialize Prometheus metrics
	grpc_prometheus.Register(srv)

	return &Server{
		server:  srv,
		logger:  logger,
		metrics: m,
		port:    port,
	}
}

// Start begins listening for gRPC requests
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	s.logger.Info("Starting gRPC server", zap.String("addr", addr))
	go func() {
		if err := s.server.Serve(listener); err != nil {
			s.logger.Error("Failed to serve gRPC", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server
func (s *Server) Stop() {
	s.logger.Info("Stopping gRPC server")
	s.server.GracefulStop()
}

// GetServer returns the underlying gRPC server instance
func (s *Server) GetServer() *grpc.Server {
	return s.server
}
