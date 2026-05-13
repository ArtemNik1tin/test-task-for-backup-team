package resolver

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ArtemNik1tin/dns-manager/api/internal/domain"
)

func newTestResolver(t *testing.T, lines ...string) *Resolver {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolv.conf")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	for _, line := range lines {
		_, err := f.WriteString(line + "\n")
		if err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}

	return NewResolver(path)
}

func TestResolver_List_EmptyFile(t *testing.T) {
	r := newTestResolver(t)
	servers, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestResolver_List_SingleServer(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8")
	servers, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].String() != "8.8.8.8" {
		t.Errorf("expected 8.8.8.8, got %s", servers[0].String())
	}
}

func TestResolver_List_MultipleServers(t *testing.T) {
	r := newTestResolver(t,
		"nameserver 8.8.8.8",
		"nameserver 1.1.1.1",
		"nameserver 8.8.4.4",
	)
	servers, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}
}

func TestResolver_List_IgnoresComments(t *testing.T) {
	r := newTestResolver(t,
		"# this is a comment",
		"nameserver 8.8.8.8",
		"; also a comment",
		"nameserver 1.1.1.1",
	)
	servers, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestResolver_List_ErrorOnInvalidIP(t *testing.T) {
	r := newTestResolver(t, "nameserver bad-ip")
	_, err := r.List(context.Background())
	if err == nil {
		t.Error("expected error for invalid IP, got nil")
	}
}

func TestResolver_List_FileNotFound(t *testing.T) {
	r := NewResolver("/nonexistent/resolv.conf")
	_, err := r.List(context.Background())
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestResolver_Add_NewServer(t *testing.T) {
	r := newTestResolver(t)
	ns, _ := domain.NewNameserver("8.8.8.8")

	err := r.Add(context.Background(), ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
}

func TestResolver_Add_Duplicate(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8")
	ns, _ := domain.NewNameserver("8.8.8.8")

	err := r.Add(context.Background(), ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 1 {
		t.Errorf("expected 1 server (no duplicate), got %d", len(servers))
	}
}

func TestResolver_Add_ToExistingFile(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8")
	ns, _ := domain.NewNameserver("1.1.1.1")

	err := r.Add(context.Background(), ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestResolver_Delete_Existing(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8", "nameserver 1.1.1.1")
	ns, _ := domain.NewNameserver("8.8.8.8")

	err := r.Delete(context.Background(), ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].String() != "1.1.1.1" {
		t.Errorf("expected 1.1.1.1, got %s", servers[0].String())
	}
}

func TestResolver_Delete_NotExists(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8")
	ns, _ := domain.NewNameserver("1.1.1.1")

	err := r.Delete(context.Background(), ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 1 {
		t.Fatalf("expected 1 server (unchanged), got %d", len(servers))
	}
}

func TestResolver_Delete_AllServers(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8")
	ns, _ := domain.NewNameserver("8.8.8.8")

	err := r.Delete(context.Background(), ns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestResolver_Delete_FileNotFound(t *testing.T) {
	r := NewResolver("/nonexistent/resolv.conf")
	ns, _ := domain.NewNameserver("8.8.8.8")

	err := r.Delete(context.Background(), ns)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestResolver_ConcurrentAccess(t *testing.T) {
	r := newTestResolver(t)
	ctx := context.Background()
	var wg sync.WaitGroup

	// Add servers concurrently.
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "8.8.4.4"} {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			ns, _ := domain.NewNameserver(ip)
			_ = r.Add(ctx, ns)
		}(ip)
	}
	wg.Wait()

	servers, err := r.List(ctx)
	if err != nil {
		t.Fatalf("list after concurrent add: %v", err)
	}
	if len(servers) != 3 {
		t.Errorf("expected 3 servers after concurrent add, got %d", len(servers))
	}
}

func TestResolver_AtomicRewrite_DataIntegrity(t *testing.T) {
	r := newTestResolver(t,
		"nameserver 8.8.8.8",
		"nameserver 1.1.1.1",
		"nameserver 8.8.4.4",
	)
	ns, _ := domain.NewNameserver("1.1.1.1")

	err := r.Delete(context.Background(), ns)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	servers, _ := r.List(context.Background())
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers after rewrite, got %d", len(servers))
	}

	// Verify the file on disk has correct content.
	data, err := os.ReadFile(r.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	expected := "nameserver 8.8.8.8\nnameserver 8.8.4.4\n"
	if string(data) != expected {
		t.Errorf("unexpected file content:\ngot:  %q\nwant: %q", string(data), expected)
	}
}

func TestResolver_FilePermissions(t *testing.T) {
	r := newTestResolver(t)
	ns, _ := domain.NewNameserver("8.8.8.8")

	err := r.Add(context.Background(), ns)
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	info, err := os.Stat(r.path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("expected 0644 permissions, got %o", info.Mode().Perm())
	}
}

func TestResolver_Add_MultipleConcurrent(t *testing.T) {
	r := newTestResolver(t)
	ctx := context.Background()
	var wg sync.WaitGroup

	ips := []string{"8.8.8.8", "1.1.1.1", "8.8.4.4", "9.9.9.9", "208.67.222.222"}
	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			ns, _ := domain.NewNameserver(ip)
			_ = r.Add(ctx, ns)
		}(ip)
	}
	wg.Wait()

	servers, err := r.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(servers) != len(ips) {
		t.Errorf("expected %d servers, got %d", len(ips), len(servers))
	}
}

func TestResolver_ReadAfterWrite(t *testing.T) {
	r := newTestResolver(t)
	ctx := context.Background()

	ns1, _ := domain.NewNameserver("8.8.8.8")
	ns2, _ := domain.NewNameserver("1.1.1.1")

	_ = r.Add(ctx, ns1)
	_ = r.Add(ctx, ns2)

	servers, err := r.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestResolver_Delete_RemovesFromDisk(t *testing.T) {
	r := newTestResolver(t, "nameserver 8.8.8.8")
	ctx := context.Background()

	ns, _ := domain.NewNameserver("8.8.8.8")
	_ = r.Delete(ctx, ns)

	data, err := os.ReadFile(r.path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file after deleting only server, got %q", string(data))
	}
}

func TestResolver_List_EmptyLines(t *testing.T) {
	r := newTestResolver(t,
		"",
		"nameserver 8.8.8.8",
		"",
		"nameserver 1.1.1.1",
		"",
	)
	servers, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}
}
