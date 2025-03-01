package mirror_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/index-provider/engine"
	"github.com/filecoin-project/index-provider/metadata"
	"github.com/filecoin-project/index-provider/mirror"
	"github.com/filecoin-project/index-provider/testutil"
	"github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

const (
	testContextTimeout  = 10 * time.Second
	testEventualTimeout = testContextTimeout / 2
	testCheckInterval   = testEventualTimeout / 10
	testRandomSeed      = 1413
)

func newTestContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	t.Cleanup(cancel)
	return ctx
}

func TestMirror_PutAdIsMirrored(t *testing.T) {
	ctx := newTestContext(t)
	rng := rand.New(rand.NewSource(testRandomSeed))
	wantMhs := testutil.RandomMultihashes(t, rng, 42)
	wantCtxID := []byte("fish")
	wantMetadata := metadata.New(metadata.Bitswap{}, &metadata.GraphsyncFilecoinV1{
		PieceCID:     testutil.RandomCids(t, rng, 1)[0],
		VerifiedDeal: true,
	})

	te := &testEnv{}
	// Start original index provider
	te.startSource(t, ctx, engine.WithPublisherKind(engine.DataTransferPublisher))

	// Publish an advertisement on original provider.
	originalAdCid := te.putAdOnSource(t, ctx, wantCtxID, wantMhs, wantMetadata)

	// Start a mirror for the original provider with reduced tick time for faster test turnaround.
	te.startMirror(t, ctx, mirror.WithSyncInterval(time.NewTicker(time.Second)))

	// Eventually require some head ad CID at the mirror.
	var gotMirroredHeadCid cid.Cid
	var err error
	require.Eventually(t, func() bool {
		gotMirroredHeadCid, err = te.mirrorSyncer.GetHead(ctx)
		return err == nil && !cid.Undef.Equals(gotMirroredHeadCid)
	}, testEventualTimeout, testCheckInterval, "err: %v", err)

	// Assert mirrored correctly.
	te.requireAdChainMirroredRecursively(t, ctx, originalAdCid, gotMirroredHeadCid)
}

func TestMirror_IsAlsoCdnForOriginalAds(t *testing.T) {
	ctx := newTestContext(t)
	rng := rand.New(rand.NewSource(testRandomSeed))
	md := metadata.New(metadata.Bitswap{})

	te := &testEnv{}
	// Start original index provider
	te.startSource(t, ctx, engine.WithPublisherKind(engine.DataTransferPublisher))

	// Publish a bunch of ads on the original provider
	ad1 := te.putAdOnSource(t, ctx, []byte("ad1"), testutil.RandomMultihashes(t, rng, 3), md)
	ad2 := te.putAdOnSource(t, ctx, []byte("ad2"), testutil.RandomMultihashes(t, rng, 4), md)
	ad3 := te.putAdOnSource(t, ctx, []byte("ad3"), testutil.RandomMultihashes(t, rng, 5), md)
	ad4 := te.removeAdOnSource(t, ctx, []byte("ad1"))

	// Start a mirror for the original provider with reduced tick time for faster test turnaround.
	te.startMirror(t, ctx, mirror.WithSyncInterval(time.NewTicker(time.Second)))

	// Eventually require all original ads to be retrievable from the mirror.
	var err error
	require.Eventually(t, func() bool {
		if err = te.mirrorSyncer.Sync(ctx, ad1, selectorparse.CommonSelector_MatchPoint); err != nil {
			return false
		}
		if err = te.mirrorSyncer.Sync(ctx, ad2, selectorparse.CommonSelector_MatchPoint); err != nil {
			return false
		}
		if err = te.mirrorSyncer.Sync(ctx, ad3, selectorparse.CommonSelector_MatchPoint); err != nil {
			return false
		}
		if err = te.mirrorSyncer.Sync(ctx, ad4, selectorparse.CommonSelector_MatchPoint); err != nil {
			return false
		}
		return true
	}, testEventualTimeout, testCheckInterval, "err: %v", err)
}

func TestMirror_FormsExpectedAdChain(t *testing.T) {
	ctx := newTestContext(t)
	rng := rand.New(rand.NewSource(testRandomSeed))
	md := metadata.New(metadata.Bitswap{})

	te := &testEnv{}
	// Start original index provider
	te.startSource(t, ctx, engine.WithPublisherKind(engine.DataTransferPublisher))

	// Publish a bunch of ads on the original provider
	_ = te.putAdOnSource(t, ctx, []byte("ad1"), testutil.RandomMultihashes(t, rng, 3), md)
	_ = te.putAdOnSource(t, ctx, []byte("ad2"), testutil.RandomMultihashes(t, rng, 4), md)
	_ = te.putAdOnSource(t, ctx, []byte("ad3"), testutil.RandomMultihashes(t, rng, 5), md)
	originalHeadAdCid := te.removeAdOnSource(t, ctx, []byte("ad1"))

	// Start a mirror for the original provider with reduced tick time for faster test turnaround.
	te.startMirror(t, ctx, mirror.WithSyncInterval(time.NewTicker(time.Second)))

	// Await until the entire chain is mirrored; this is done by checking if the head mirrored ad
	// is a removal.
	var gotMirroredHeadAdCid cid.Cid
	var err error
	require.Eventually(t, func() bool {
		gotMirroredHeadAdCid, err = te.mirrorSyncer.GetHead(ctx)
		if err != nil || cid.Undef.Equals(gotMirroredHeadAdCid) {
			return false
		}
		ad, err := te.syncMirrorAd(ctx, gotMirroredHeadAdCid)
		if err != nil {
			return false
		}
		// The head ad should be a removal since that's the last ad published by the original
		// provider.
		return ad.IsRm
	}, testEventualTimeout, testCheckInterval, "err: %v", err)

	// Read each individual mirrored ad and assert it is mirrored as expected.
	te.requireAdChainMirroredRecursively(t, ctx, originalHeadAdCid, gotMirroredHeadAdCid)
}

func TestMirror_FormsExpectedAdChainRemap(t *testing.T) {
	tests := []struct {
		name          string
		mirrorOptions []mirror.Option
	}{
		{
			name: "unchanged",
		},
		{
			name:          "hamt_murmur_3_3",
			mirrorOptions: []mirror.Option{mirror.WithHamtRemapper(multihash.MURMUR3X64_64, 3, 3)},
		},
		{
			name:          "hamt_id_3_1",
			mirrorOptions: []mirror.Option{mirror.WithHamtRemapper(multihash.IDENTITY, 3, 1)},
		},
		{
			name:          "entry_chunk_1",
			mirrorOptions: []mirror.Option{mirror.WithEntryChunkRemapper(1)},
		},
		{
			name:          "entry_chunk_1000",
			mirrorOptions: []mirror.Option{mirror.WithEntryChunkRemapper(1000)},
		},
		{
			name:          "hamt_murmur_3_3_reSign",
			mirrorOptions: []mirror.Option{mirror.WithHamtRemapper(multihash.MURMUR3X64_64, 3, 3), mirror.WithAlwaysReSignAds(true)},
		},
		{
			name:          "hamt_id_3_1_reSign",
			mirrorOptions: []mirror.Option{mirror.WithHamtRemapper(multihash.IDENTITY, 3, 1), mirror.WithAlwaysReSignAds(true)},
		},
		{
			name:          "entry_chunk_1_reSign",
			mirrorOptions: []mirror.Option{mirror.WithEntryChunkRemapper(1), mirror.WithAlwaysReSignAds(true)},
		},
		{
			name:          "entry_chunk_1000_reSign",
			mirrorOptions: []mirror.Option{mirror.WithEntryChunkRemapper(1000), mirror.WithAlwaysReSignAds(true)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := newTestContext(t)
			rng := rand.New(rand.NewSource(testRandomSeed))
			md := metadata.New(metadata.Bitswap{})

			te := &testEnv{}
			// Start original index provider
			te.startSource(t, ctx, engine.WithPublisherKind(engine.DataTransferPublisher))

			// Publish a bunch of ads on the original provider
			_ = te.putAdOnSource(t, ctx, []byte("ad1"), testutil.RandomMultihashes(t, rng, 1), md)
			_ = te.putAdOnSource(t, ctx, []byte("ad2"), testutil.RandomMultihashes(t, rng, 400), md)
			_ = te.removeAdOnSource(t, ctx, []byte("ad1"))
			_ = te.putAdOnSource(t, ctx, []byte("ad3"), testutil.RandomMultihashes(t, rng, 1), md)
			_ = te.putAdOnSource(t, ctx, []byte("ad4"), testutil.RandomMultihashes(t, rng, 2), md)
			_ = te.removeAdOnSource(t, ctx, []byte("ad2"))
			originalHeadAdCid := te.putAdOnSource(t, ctx, []byte("ad5"), testutil.RandomMultihashes(t, rng, 7), md)

			test.mirrorOptions = append(test.mirrorOptions, mirror.WithSyncInterval(time.NewTicker(time.Second)))
			te.startMirror(t, ctx, test.mirrorOptions...)

			// Await until the entire chain is mirrored; this is done by checking if the head mirrored ad
			// is a removal.
			var gotMirroredHeadAdCid cid.Cid
			var err error
			require.Eventually(t, func() bool {
				gotMirroredHeadAdCid, err = te.mirrorSyncer.GetHead(ctx)
				if err != nil || cid.Undef.Equals(gotMirroredHeadAdCid) {
					return false
				}
				// Check the context is the latest originally published ad context as a way to
				// assert that the entire ad chain is mirrored.
				var ad *schema.Advertisement
				ad, err = te.syncMirrorAd(ctx, gotMirroredHeadAdCid)
				return err == nil && string(ad.ContextID) == "ad5"
			}, testEventualTimeout, testCheckInterval, "err: %v", err)

			// Read each individual mirrored ad and assert it is mirrored as expected.
			te.requireAdChainMirroredRecursively(t, ctx, originalHeadAdCid, gotMirroredHeadAdCid)
		})
	}
}

func TestMirror_PreviousIDIsPreservedOnStartFromPartialAdChain(t *testing.T) {
	ctx := newTestContext(t)
	rng := rand.New(rand.NewSource(testRandomSeed))
	md := metadata.New(metadata.Bitswap{})

	te := &testEnv{}
	// Start source and publish 3 ads.
	te.startSource(t, ctx, engine.WithPublisherKind(engine.DataTransferPublisher))
	originalACid := te.putAdOnSource(t, ctx, []byte("ad1"), testutil.RandomMultihashes(t, rng, 1), md)
	originalBCid := te.putAdOnSource(t, ctx, []byte("ad2"), testutil.RandomMultihashes(t, rng, 2), md)
	orignalHeadCid := te.putAdOnSource(t, ctx, []byte("ad3"), testutil.RandomMultihashes(t, rng, 3), md)

	// Start mirror with maximum initial depth of 2.
	te.startMirror(t, ctx, mirror.WithSyncInterval(time.NewTicker(time.Second)), mirror.WithInitialAdRecursionLimit(selector.RecursionLimitDepth(2)))

	var gotMirroredHeadAdCid cid.Cid
	var err error
	require.Eventually(t, func() bool {
		gotMirroredHeadAdCid, err = te.mirrorSyncer.GetHead(ctx)
		if err != nil || cid.Undef.Equals(gotMirroredHeadAdCid) {
			return false
		}
		// Check the context is the latest originally published ad context as a way to
		// assert that the entire ad chain is mirrored.
		var ad *schema.Advertisement
		ad, err = te.syncMirrorAd(ctx, gotMirroredHeadAdCid)
		return err == nil && string(ad.ContextID) == "ad3"
	}, testEventualTimeout, testCheckInterval, "err: %v", err)

	// Load head and assert it is mirrored correctly.
	original, err := te.source.GetAdv(ctx, orignalHeadCid)
	require.NoError(t, err)
	mirrored, err := te.syncMirrorAd(ctx, gotMirroredHeadAdCid)
	require.NoError(t, err)
	te.requireAdMirrored(t, ctx, original, mirrored)

	// Load ad before head and assert it is mirrored
	original, err = te.source.GetAdv(ctx, originalBCid)
	require.NoError(t, err)
	mirrored, err = te.syncMirrorAd(ctx, mirrored.PreviousID.(cidlink.Link).Cid)
	require.NoError(t, err)
	te.requireAdMirrored(t, ctx, original, mirrored)

	// Assert mirrored previousID is same as original
	require.Equal(t, original.PreviousID, mirrored.PreviousID)

	// Assert the mirror does not store the earliest ad nor is a CDN for it.
	// Note that we can't explicitly assert not found error since the final error returned depends
	// entirely on the order of concurrent interactions and based on the intermittent CI failures it
	// is not reproducible.
	err = te.syncFromMirrorRecursively(ctx, mirrored.PreviousID.(cidlink.Link).Cid)
	require.NotNil(t, err)
	err = te.syncFromMirrorRecursively(ctx, originalACid)
	require.NotNil(t, err)
}

func TestMirror_MirrorsAdsIdenticallyWhenConfiguredTo(t *testing.T) {
	ctx := newTestContext(t)
	rng := rand.New(rand.NewSource(testRandomSeed))
	md := metadata.New(metadata.Bitswap{})

	te := &testEnv{}
	// Start source and publish 3 ads.
	te.startSource(t, ctx, engine.WithPublisherKind(engine.DataTransferPublisher))
	_ = te.putAdOnSource(t, ctx, []byte("ad1"), testutil.RandomMultihashes(t, rng, 1), md)
	_ = te.putAdOnSource(t, ctx, []byte("ad2"), testutil.RandomMultihashes(t, rng, 2), md)
	_ = te.removeAdOnSource(t, ctx, []byte("ad1"))
	originalHeadCid := te.putAdOnSource(t, ctx, []byte("ad3"), testutil.RandomMultihashes(t, rng, 3), md)

	te.startMirror(t, ctx, mirror.WithSyncInterval(time.NewTicker(time.Second)), mirror.WithAlwaysReSignAds(false))

	var gotMirroredHeadAdCid cid.Cid
	var err error
	require.Eventually(t, func() bool {
		gotMirroredHeadAdCid, err = te.mirrorSyncer.GetHead(ctx)
		if err != nil || cid.Undef.Equals(gotMirroredHeadAdCid) {
			return false
		}
		// Check that head CID in mirror is the same as original head cid sicne ad chain must be
		// identical.
		_, err = te.syncMirrorAd(ctx, gotMirroredHeadAdCid)
		return err == nil && originalHeadCid.Equals(gotMirroredHeadAdCid)
	}, testEventualTimeout, testCheckInterval, "err: %v", err)

	// Load head and assert it is mirrored correctly.
	// Note that since head CIDs are equal and assertions from mirror actually sync data over
	// graphsync, then it means the remaining ad chain must be the same since CIDs are implicitly
	// verified against the content.
	te.requireAdChainMirroredRecursively(t, ctx, originalHeadCid, gotMirroredHeadAdCid)
}
