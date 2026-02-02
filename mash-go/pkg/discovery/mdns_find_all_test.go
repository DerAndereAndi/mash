package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mash-protocol/mash-go/pkg/discovery"
)

// TestFindAllByDiscriminator_Timeout verifies that FindAllByDiscriminator returns
// an empty slice (not an error) when no devices appear before the context expires.
func TestFindAllByDiscriminator_Timeout(t *testing.T) {
	config := testBrowserConfig(t)
	browser, err := discovery.NewMDNSBrowser(config)
	if err != nil {
		t.Fatalf("NewMDNSBrowser() error = %v", err)
	}
	defer browser.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	results, err := browser.FindAllByDiscriminator(ctx, 1234)
	assert.NoError(t, err)
	assert.Empty(t, results, "should return empty slice when no devices found")
}

// TestFindAllByDiscriminator_ContextCancelled verifies that cancelling the context
// returns whatever was collected so far (empty in this case).
func TestFindAllByDiscriminator_ContextCancelled(t *testing.T) {
	config := testBrowserConfig(t)
	browser, err := discovery.NewMDNSBrowser(config)
	if err != nil {
		t.Fatalf("NewMDNSBrowser() error = %v", err)
	}
	defer browser.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancel()

	results, err := browser.FindAllByDiscriminator(ctx, 1234)
	assert.NoError(t, err)
	assert.Empty(t, results, "should return empty slice on immediate cancel")
}
