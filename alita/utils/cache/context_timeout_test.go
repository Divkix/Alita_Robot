package cache

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/divkix/Alita_Robot/alita/config"
)

// stallProxy forwards TCP traffic to a real Redis until stall is called. After
// that it swallows every request, so the server never answers and a client
// waits on a reply that never comes: a Redis that hangs mid-command.
type stallProxy struct {
	ln      net.Listener
	target  string
	stalled atomic.Bool
	mu      sync.Mutex
	conns   []net.Conn
}

func newStallProxy(t *testing.T, target string) *stallProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &stallProxy{ln: ln, target: target}
	go p.serve()
	t.Cleanup(p.close)
	return p
}

func (p *stallProxy) Addr() string { return p.ln.Addr().String() }

func (p *stallProxy) stall() { p.stalled.Store(true) }

func (p *stallProxy) track(c net.Conn) {
	p.mu.Lock()
	p.conns = append(p.conns, c)
	p.mu.Unlock()
}

func (p *stallProxy) close() {
	_ = p.ln.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
}

func (p *stallProxy) serve() {
	for {
		client, err := p.ln.Accept()
		if err != nil {
			return
		}
		server, err := net.Dial("tcp", p.target)
		if err != nil {
			_ = client.Close()
			continue
		}
		p.track(client)
		p.track(server)
		go func() { _, _ = io.Copy(client, server) }()
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := client.Read(buf)
				if n > 0 && !p.stalled.Load() {
					if _, werr := server.Write(buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()
	}
}

// The bot bounds Redis calls with context deadlines (loader reads, DeleteCache,
// the health probe). The shared client must stop at that deadline instead of
// waiting out its own ReadTimeout, or a hung Redis stalls every caller for
// seconds past the budget it asked for.
func TestRedisClientHonoursContextDeadline(t *testing.T) {
	tests := []struct {
		name   string
		config func(addr string) *config.Config
	}{
		{"address", func(addr string) *config.Config { return &config.Config{RedisAddress: addr} }},
		{"url", func(addr string) *config.Config { return &config.Config{RedisURL: "redis://" + addr + "/0"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDIS_PASSWORD", "")
			t.Setenv("REDIS_DB", "")
			server := miniredis.RunT(t)
			proxy := newStallProxy(t, server.Addr())

			originalConfig := config.AppConfig
			originalMarshal, originalManager, originalClient := GetCacheState()
			config.AppConfig = tt.config(proxy.Addr())
			t.Cleanup(func() {
				if client := GetRedisClient(); client != nil {
					_ = client.Close()
				}
				config.AppConfig = originalConfig
				SetCacheState(originalMarshal, originalManager, originalClient)
			})
			if err := InitCache(); err != nil {
				t.Fatalf("InitCache() error = %v", err)
			}
			client := GetRedisClient()
			if err := client.Set(Context, "alita:cache:probe", "v", 0).Err(); err != nil {
				t.Fatalf("Set before stall: %v", err)
			}

			proxy.stall()
			const budget = 200 * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			start := time.Now()
			err := client.Get(ctx, "alita:cache:probe").Err()
			elapsed := time.Since(start)

			if err == nil {
				t.Fatal("Get on a hung Redis returned no error")
			}
			if elapsed > 2*time.Second {
				t.Fatalf("Get with a %v deadline returned after %v (err %v); the client ignored the context deadline", budget, elapsed, err)
			}
			var netErr net.Error
			isTimeout := errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout())
			if !isTimeout {
				t.Fatalf("Get error = %v, want a timeout", err)
			}
		})
	}
}
