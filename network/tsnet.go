package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"path/filepath"

	"github.com/MattCruikshank/tsipfs/config"
	"tailscale.com/tsnet"
)

type TailscaleNode struct {
	Server        *tsnet.Server
	FunnelListener net.Listener
	AdminListener  net.Listener
}

func StartTailscale(ctx context.Context, cfg *config.Config) (*TailscaleNode, error) {
	hostname := cfg.FunnelHostname
	if hostname == "" {
		hostname = "tsipfs"
	}

	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      filepath.Join(cfg.DataDir, "tsnet"),
		AuthKey:  cfg.TSAuthKey,
	}

	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("starting tsnet server: %w", err)
	}
	log.Printf("tsnet server started as %s", hostname)

	// Funnel listener for the public IPFS gateway (HTTPS on port 443)
	funnelLn, err := srv.ListenFunnel("tcp", ":443")
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("starting funnel listener: %w", err)
	}
	log.Println("funnel listener active on :443")

	// Tailnet-only listener for admin UI and REST API
	adminLn, err := srv.Listen("tcp", fmt.Sprintf(":%d", cfg.AdminPort))
	if err != nil {
		funnelLn.Close()
		srv.Close()
		return nil, fmt.Errorf("starting admin listener: %w", err)
	}
	log.Printf("admin listener active on :%d (tailnet-only)", cfg.AdminPort)

	return &TailscaleNode{
		Server:         srv,
		FunnelListener: funnelLn,
		AdminListener:  adminLn,
	}, nil
}

func (t *TailscaleNode) Close() error {
	if t.FunnelListener != nil {
		t.FunnelListener.Close()
	}
	if t.AdminListener != nil {
		t.AdminListener.Close()
	}
	return t.Server.Close()
}
