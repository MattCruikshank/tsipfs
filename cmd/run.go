package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/MattCruikshank/tsipfs/api"
	"github.com/MattCruikshank/tsipfs/config"
	"github.com/MattCruikshank/tsipfs/network"
	"github.com/MattCruikshank/tsipfs/node"
	"github.com/MattCruikshank/tsipfs/ui"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the tsipfs node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.SwarmKey == "" {
			return fmt.Errorf("TSIPFS_SWARM_KEY is required. Generate one with: tsipfs generate-swarm-key")
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		startTime := time.Now()

		// Start Tailscale networking
		ts, err := network.StartTailscale(ctx, cfg)
		if err != nil {
			return fmt.Errorf("starting tailscale: %w", err)
		}
		defer ts.Close()

		// Start IPFS node
		ipfsNode, err := node.Start(ctx, cfg)
		if err != nil {
			return fmt.Errorf("starting IPFS node: %w", err)
		}
		defer ipfsNode.Close()

		// Start cache evictor
		evictor := node.NewCacheEvictor(
			filepath.Join(cfg.DataDir, "cache"),
			cfg.CacheMaxSize,
		)
		go evictor.Run(ctx)

		// Build gateway URL from the Funnel cert domains
		gatewayURL := ""
		if domains := ts.Server.CertDomains(); len(domains) > 0 {
			gatewayURL = fmt.Sprintf("https://%s", domains[0])
		}

		// Create API router
		apiRouter := api.NewRouter(ipfsNode, startTime, gatewayURL)

		// Public IPFS gateway with rate limiting
		gwHandler, err := ipfsNode.GatewayHandler()
		if err != nil {
			return fmt.Errorf("creating gateway handler: %w", err)
		}
		// 100 requests per second per IP, burst of 200
		rateLimiter := api.NewRateLimiter(100, time.Second, 200)
		rateLimitedGW := rateLimiter.Middleware(gwHandler)

		gatewaySrv := &http.Server{Handler: rateLimitedGW}
		go func() {
			log.Println("IPFS gateway serving on Funnel (public, rate-limited)")
			if err := gatewaySrv.Serve(ts.FunnelListener); err != nil && err != http.ErrServerClosed {
				log.Printf("gateway server error: %v", err)
			}
		}()

		// Admin API + UI on tailnet-only listener (with Tailscale auth)
		adminMux := http.NewServeMux()
		adminMux.Handle("/api/", apiRouter)
		adminMux.Handle("/", ui.Handler())
		tailnetAPI := api.TailscaleAuth(ts.Server, adminMux)

		adminSrv := &http.Server{Handler: tailnetAPI}
		go func() {
			log.Printf("admin API serving on :%d (tailnet-only)", cfg.AdminPort)
			if err := adminSrv.Serve(ts.AdminListener); err != nil && err != http.ErrServerClosed {
				log.Printf("admin server error: %v", err)
			}
		}()

		// Unix socket for local CLI access
		sockPath := filepath.Join(cfg.DataDir, "api.sock")
		os.Remove(sockPath)
		sockLn, err := net.Listen("unix", sockPath)
		if err != nil {
			return fmt.Errorf("creating unix socket: %w", err)
		}

		localMux := http.NewServeMux()
		localMux.Handle("/api/", apiRouter)
		localMux.Handle("/ipfs/", gwHandler)
		localAPI := api.LocalAuth(localMux)

		localSrv := &http.Server{Handler: localAPI}
		go func() {
			log.Printf("local API socket: %s", sockPath)
			if err := localSrv.Serve(sockLn); err != nil && err != http.ErrServerClosed {
				log.Printf("local API server error: %v", err)
			}
		}()

		log.Println("tsipfs node is running")

		// Wait for shutdown signal
		<-ctx.Done()
		log.Println("shutting down...")

		// Graceful shutdown with 15 second timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()

		// Shut down HTTP servers in parallel
		done := make(chan struct{})
		go func() {
			defer close(done)
			if err := gatewaySrv.Shutdown(shutdownCtx); err != nil {
				log.Printf("gateway shutdown: %v", err)
			}
			if err := adminSrv.Shutdown(shutdownCtx); err != nil {
				log.Printf("admin shutdown: %v", err)
			}
			if err := localSrv.Shutdown(shutdownCtx); err != nil {
				log.Printf("local API shutdown: %v", err)
			}
		}()

		select {
		case <-done:
			log.Println("HTTP servers stopped gracefully")
		case <-shutdownCtx.Done():
			log.Println("shutdown timed out, forcing exit")
		}

		// Clean up socket
		os.Remove(sockPath)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
