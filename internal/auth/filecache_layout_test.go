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
// azidentity v1.13.1.
//
// What this test can and cannot see. azidentity's internal.impl lives in
// azidentity/internal, which is unimportable, so nothing here can compare
// against it directly. What is checked is that azidentity.Cache stays one
// pointer wide, and that cacheImplShim's own field layout does not drift. That
// catches a local edit to the shim, and it catches Cache gaining a field --
// but a field added or reordered inside internal.impl on an azidentity upgrade
// will pass this test silently. Bumping azidentity still requires reading
// internal.impl by hand.
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

	// Offsets of the mirrored impl struct, derived from the field widths rather
	// than hardcoded, so the test states the layout rule instead of one
	// architecture's arithmetic. factory is a func (1 word), cae and noCAE are
	// interfaces (2 words each), mu is a pointer (1 word). Hardcoding 0/8/24/40
	// would fail on a 32-bit build of a perfectly sound cast.
	var (
		word              = unsafe.Sizeof(uintptr(0))
		iface             = unsafe.Sizeof(msalcache.ExportReplace(nil))
		wantFactoryOffset = uintptr(0)
		wantCAEOffset     = word
		wantNoCAEOffset   = word + iface
		wantMuOffset      = word + 2*iface
		wantImplSize      = word + 2*iface + word
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

	// The field widths the offsets above are derived from. An interface header
	// is two words and a pointer is one; if either stops being true the derived
	// offsets are meaningless, so say which assumption moved.
	if iface != 2*word {
		t.Errorf("sizeof(ExportReplace) = %d, want %d (two-word interface header)", iface, 2*word)
	}
	if got := unsafe.Sizeof((*sync.RWMutex)(nil)); got != word {
		t.Errorf("sizeof(*sync.RWMutex) = %d, want %d (one-word pointer)", got, word)
	}
}
