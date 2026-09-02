// Package auth per-request account hand-back.
//
// This file solves a middleware ordering problem. AuthMiddleware wraps
// AccountResolver, not the other way round (see the wrap and wrapWrite helpers
// in internal/server/verbs_config.go, which compose
// authMW(accountResolverMW(...))). AccountResolver injects the resolved account
// with WithAccountAuth into a context it derives *inside* that call, so the
// derived value can never travel back out to AuthMiddleware. The consequence
// before this change was that handleAuthError's AccountAuthFromContext lookup
// always missed and re-authentication always targeted the server's default
// credential, whichever account the tool call actually used.
//
// The fix is a mutable slot: AuthMiddleware allocates one and puts a pointer
// to it in the context before calling the handler chain; AccountResolver fills
// it in; AuthMiddleware reads it afterwards. Passing a pointer through context
// is the standard Go workaround for handing a value back up a middleware
// chain.
//
// Ported from outlook-local-mcp CR-0067 A6, which this repository's CR-0067
// recorded as a follow-up rather than porting at the time.
package auth

import (
	"context"
	"sync"
)

// accountAuthSlot is a mutable, concurrency-safe container for the AccountAuth
// that AccountResolver resolved for one tool call.
//
// A tool handler runs on a single goroutine, but the slot is guarded anyway:
// the interactive flows read it from a background goroutine, so the mutex is
// load-bearing rather than merely documentary.
type accountAuthSlot struct {
	// mu guards auth and set.
	mu sync.Mutex

	// auth is the resolved account's authentication details. Meaningful only
	// when set is true.
	auth AccountAuth

	// set records whether store has been called.
	set bool
}

// store records the account details resolved for this request.
//
// Parameters:
//   - auth: the resolved account's authenticator, auth record path and method.
//
// Side effects: overwrites any previously stored value.
func (s *accountAuthSlot) store(auth AccountAuth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auth = auth
	s.set = true
}

// load returns the stored account details.
//
// Returns the AccountAuth and true when store has been called, or a zero value
// and false otherwise. No side effects.
func (s *accountAuthSlot) load() (AccountAuth, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.auth, s.set
}

// accountAuthSlotKeyType is the unexported context key type for the slot.
type accountAuthSlotKeyType struct{}

// accountAuthSlotKey is the package-level context key for slot storage.
var accountAuthSlotKey = accountAuthSlotKeyType{}

// withAccountAuthSlot returns a context carrying a freshly allocated slot,
// along with a pointer to that slot for the caller to read later.
//
// Parameters:
//   - ctx: the parent context.
//
// Returns the derived context and the slot pointer.
func withAccountAuthSlot(ctx context.Context) (context.Context, *accountAuthSlot) {
	slot := &accountAuthSlot{}
	return context.WithValue(ctx, accountAuthSlotKey, slot), slot
}

// accountAuthSlotFromContext retrieves the slot installed by
// withAccountAuthSlot. AccountResolver calls this to report the account it
// resolved back to AuthMiddleware.
//
// Parameters:
//   - ctx: the context to look in.
//
// Returns the slot and true when one is present, or nil and false otherwise
// (for example when AccountResolver is used without AuthMiddleware, as in
// unit tests).
func accountAuthSlotFromContext(ctx context.Context) (*accountAuthSlot, bool) {
	if ctx == nil {
		return nil, false
	}
	slot, ok := ctx.Value(accountAuthSlotKey).(*accountAuthSlot)
	return slot, ok
}

// resolvedAccountAuth returns the account details for the current request,
// preferring the slot filled in by AccountResolver and falling back to a
// value placed directly in the context with WithAccountAuth.
//
// The slot takes precedence because it reflects the account the handler chain
// actually resolved for this call, whereas a direct context value can only
// have been set by a caller upstream of the middleware.
//
// Parameters:
//   - ctx: the tool handler context.
//
// Returns the AccountAuth and true when either source has a value, or a zero
// value and false when neither does. No side effects.
func resolvedAccountAuth(ctx context.Context) (AccountAuth, bool) {
	if slot, ok := accountAuthSlotFromContext(ctx); ok {
		if auth, filled := slot.load(); filled {
			return auth, true
		}
	}
	return AccountAuthFromContext(ctx)
}
