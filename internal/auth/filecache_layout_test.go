package auth

import (
	"sync"
	"testing"
	"unsafe"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	msalcache "github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// TestFileCacheShimLayout is the test filecache.go's doc comment has always
// claimed verifies the shim layout. It did not exist until now, so the
// unsafe.Pointer cast in initFileCacheValue rested on an unchecked assumption
// about a third-party module's private struct.
//
// initFileCacheValue builds a cacheShim and reinterprets it as an
// azidentity.Cache. That is only sound while cacheImplShim has byte-for-byte
// the same layout as azidentity's internal.impl, pinned here at
// azidentity v1.13.1. Two things can break it: azidentity changing its own
// struct on upgrade, or someone editing cacheImplShim here.
//
// The cae and noCAE fields are never read or written by name, which makes
// linters call them unused. They are not: they occupy the offsets azidentity
// expects, and removing either one silently moves mu, so azidentity would read
// a *sync.RWMutex out of an ExportReplace slot.
func TestFileCacheShimLayout(t *testing.T) {
	// The outer cast target must stay a single pointer wide. If azidentity
	// adds a field to Cache, this is the assertion that catches it.
	if got, want := unsafe.Sizeof(cacheShim{}), unsafe.Sizeof(azidentity.Cache{}); got != want {
		t.Fatalf("sizeof(cacheShim) = %d, sizeof(azidentity.Cache) = %d: "+
			"the cast in initFileCacheValue is no longer layout-safe", got, want)
	}

	// Offsets of the mirrored impl struct, pinned to azidentity v1.13.1.
	// factory is a func (1 word), cae and noCAE are interfaces (2 words each),
	// mu is a pointer (1 word).
	const (
		wantFactoryOffset = 0
		wantCAEOffset     = 8
		wantNoCAEOffset   = 24
		wantMuOffset      = 40
		wantImplSize      = 48
	)

	checks := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"factory offset", unsafe.Offsetof(cacheImplShim{}.factory), wantFactoryOffset},
		{"cae offset", unsafe.Offsetof(cacheImplShim{}.cae), wantCAEOffset},
		{"noCAE offset", unsafe.Offsetof(cacheImplShim{}.noCAE), wantNoCAEOffset},
		{"mu offset", unsafe.Offsetof(cacheImplShim{}.mu), wantMuOffset},
		{"impl size", unsafe.Sizeof(cacheImplShim{}), wantImplSize},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d: cacheImplShim no longer mirrors "+
				"azidentity internal.impl", c.name, c.got, c.want)
		}
	}

	// The field widths the offsets above depend on. Stated explicitly so a
	// failure points at which assumption moved.
	if got := unsafe.Sizeof(msalcache.ExportReplace(nil)); got != 16 {
		t.Errorf("sizeof(ExportReplace) = %d, want 16 (interface header)", got)
	}
	if got := unsafe.Sizeof((*sync.RWMutex)(nil)); got != 8 {
		t.Errorf("sizeof(*sync.RWMutex) = %d, want 8 (pointer)", got)
	}
}
