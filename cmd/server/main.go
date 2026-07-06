// Command server runs the YouSee Musik controller web application.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danske-spil/yousee-musik-controller/internal/handlers"
	"github.com/danske-spil/yousee-musik-controller/internal/player"
	"github.com/danske-spil/yousee-musik-controller/internal/service"
	"github.com/danske-spil/yousee-musik-controller/internal/websocket"
)

func main() {
	addr := getenv("YOUSEE_ADDR", ":8080")
	username := os.Getenv("YOUSEE_USERNAME")
	password := os.Getenv("YOUSEE_PASSWORD")

	svc := service.New(username, password)
	if svc.Available() {
		log.Println("YouSee Musik service configured")
	} else {
		log.Println("No YOUSEE_USERNAME/YOUSEE_PASSWORD set — running in demo mode with mock data")
	}

	// The player is the single source of truth; the hub pushes its state to clients.
	var pl *player.Player
	hub := websocket.NewHub(func() []websocket.Event { return pl.Snapshot() })
	pl = player.New(svc, hub)

	go hub.Run()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go pl.StartTicker(ctx)

	srv, err := handlers.NewServer(svc, pl, hub)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("YouSee Musik controller listening on http://localhost%s", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
