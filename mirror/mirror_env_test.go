package mirror_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/filecoin-project/go-legs/dtsync"
	provider "github.com/filecoin-project/index-provider"
	"github.com/filecoin-project/index-provider/engine"
	"github.com/filecoin-project/index-provider/metadata"
	"github.com/filecoin-project/index-provider/mirror"
	"github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	hamt "github.com/ipld/go-ipld-adl-hamt"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	sourceHost host.Host
	source     *engine.Engine
	sourceMhs  map[string][]multihash.Multihash

	mirror            *mirror.Mirror
	mirrorHost        host.Host
	mirrorSync        *dtsync.Sync
	mirrorSyncHost    host.Host
	mirrorSyncLs      ipld.LinkSystem
	mirrorSyncer      *dtsync.Syncer
	mirrorSyncLsStore *memstore.Store
}

func (te *testEnv) startMirror(t *testing.T, ctx context.Context, opts ...mirror.Option) {
	var err error
	te.mirrorHost, err = libp2p.New()
	require.NoError(t, err)
	// Override the host, since test environment needs explicit access to it.
	opts = append(opts, mirror.WithHost(te.mirrorHost))
	te.mirror, err = mirror.New(ctx, te.sourceAddrInfo(t), opts...)
	require.NoError(t, err)
	require.NoError(t, te.mirror.Start())
	t.Cleanup(func() { require.NoError(t, te.mirror.Shutdown()) })

	te.mirrorSyncHost, err = libp2p.New()
	require.NoError(t, err)
	te.mirrorSyncHost.Peerstore().AddAddrs(te.mirrorHost.ID(), te.mirrorHost.Addrs(), peerstore.PermanentAddrTTL)

	te.mirrorSyncLsStore = &memstore.Store{}
	te.mirrorSyncLs = cidlink.DefaultLinkSystem()
	te.mirrorSyncLs.SetReadStorage(te.mirrorSyncLsStore)
	te.mirrorSyncLs.SetWriteStorage(te.mirrorSyncLsStore)

	te.mirrorSync, err = dtsync.NewSync(te.mirrorSyncHost, dssync.MutexWrap(datastore.NewMapDatastore()), te.mirrorSyncLs, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, te.mirrorSync.Close()) })
	te.mirrorSyncer = te.mirrorSync.NewSyncer(te.mirrorHost.ID(), te.mirror.GetTopicName(), nil)
}

func (te *testEnv) sourceAddrInfo(t *testing.T) peer.AddrInfo {
	require.NotNil(t, te.sourceHost, "start source first")
	return te.sourceHost.Peerstore().PeerInfo(te.sourceHost.ID())
}

func (te *testEnv) startSource(t *testing.T, ctx context.Context, opts ...engine.Option) {
	var err error
	te.sourceHost, err = libp2p.New()
	require.NoError(t, err)
	// Override the host, since test environment needs explicit access to it.
	opts = append(opts, engine.WithHost(te.sourceHost))
	te.source, err = engine.New(opts...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, te.source.Shutdown()) })
	te.sourceMhs = make(map[string][]multihash.Multihash)
	te.source.RegisterMultihashLister(te.listMultihashes)
	require.NoError(t, te.source.Start(ctx))
}

func (te *testEnv) putAdOnSource(t *testing.T, ctx context.Context, ctxID []byte, mhs []multihash.Multihash, md metadata.Metadata) cid.Cid {
	te.sourceMhs[string(ctxID)] = mhs
	adCid, err := te.source.NotifyPut(ctx, nil, ctxID, md)
	require.NoError(t, err)
	return adCid
}

func (te *testEnv) removeAdOnSource(t *testing.T, ctx context.Context, ctxID []byte) cid.Cid {
	adCid, err := te.source.NotifyRemove(ctx, "", ctxID)
	require.NoError(t, err)
	return adCid
}

func (te *testEnv) listMultihashes(_ context.Context, p peer.ID, contextID []byte) (provider.MultihashIterator, error) {
	mhs, ok := te.sourceMhs[string(contextID)]
	if !ok {
		return nil, fmt.Errorf("no multihashes found for context ID: %s", string(contextID))
	}
	return provider.SliceMultihashIterator(mhs), nil
}

func (te *testEnv) requireAdChainMirroredRecursively(t *testing.T, ctx context.Context, originalAdCid, mirroredAdCid cid.Cid) {
	// Load Ads
	original, err := te.source.GetAdv(ctx, originalAdCid)
	require.NoError(t, err)
	mirrored, err := te.syncMirrorAd(ctx, mirroredAdCid)
	require.NoError(t, err)

	te.requireAdMirrored(t, ctx, original, mirrored)

	// Assert previous ad is mirrored as expected.
	if original.PreviousID == nil {
		require.Nil(t, mirrored.PreviousID)
		return
	}
	te.requireAdChainMirroredRecursively(t, ctx, original.PreviousID.(cidlink.Link).Cid, mirrored.PreviousID.(cidlink.Link).Cid)
}

func (te *testEnv) requireAdMirrored(t *testing.T, ctx context.Context, original, mirrored *schema.Advertisement) {
	// Assert fields that should have remained the same are identical.
	require.Equal(t, original.IsRm, mirrored.IsRm)
	require.Equal(t, original.Provider, mirrored.Provider)
	require.Equal(t, original.Metadata, mirrored.Metadata)
	require.Equal(t, original.Addresses, mirrored.Addresses)
	require.Equal(t, original.ContextID, mirrored.ContextID)

	gotSigner, err := mirrored.VerifySignature()
	require.NoError(t, err)

	// In the test environment the signer of ad is either the source or the mirror depending on
	// the mirroring options or weather the PreviousID or Entries link is changed in comparison with
	// the original ad.
	// Assert one or the other accordingly.
	var wantSigner peer.ID
	if te.mirror.AlwaysReSignAds() || original.Entries != mirrored.Entries || original.PreviousID != mirrored.PreviousID {
		wantSigner = te.mirrorHost.ID()
	} else {
		wantSigner = te.sourceHost.ID()
	}
	require.Equal(t, wantSigner, gotSigner)

	// Assert entries are mirrored as expected
	te.requireEntriesMirrored(t, ctx, original.ContextID, original.Entries, mirrored.Entries)
}

func (te *testEnv) requireEntriesMirrored(t *testing.T, ctx context.Context, contextID []byte, originalEntriesLink, mirroredEntriesLink ipld.Link) {

	if originalEntriesLink == schema.NoEntries {
		require.Equal(t, schema.NoEntries, mirroredEntriesLink)
		return
	}

	// Entries link should never be nil.
	require.NotNil(t, originalEntriesLink)
	require.NotNil(t, mirroredEntriesLink)

	// Assert that the entries are sync-able from mirror which will implicitly assert that the
	// returned entries indeed correspond to the given link via block digest verification.
	err := te.mirrorSyncer.Sync(ctx, mirroredEntriesLink.(cidlink.Link).Cid, selectorparse.CommonSelector_ExploreAllRecursively)
	require.NoError(t, err)

	if !te.mirror.RemapEntriesEnabled() {
		require.Equal(t, originalEntriesLink, mirroredEntriesLink)
		return
	}

	wantMhs := te.sourceMhs[string(contextID)]

	var mirroredMhIter provider.MultihashIterator
	switch te.mirror.EntriesRemapPrototype() {
	case schema.EntryChunkPrototype:
		mirroredMhIter, err = provider.EntryChunkMultihashIterator(mirroredEntriesLink, te.mirrorSyncLs)
		require.NoError(t, err)
	case hamt.HashMapRootPrototype:
		n, err := te.mirrorSyncLs.Load(ipld.LinkContext{Ctx: ctx}, mirroredEntriesLink, hamt.HashMapRootPrototype)
		require.NoError(t, err)
		root := bindnode.Unwrap(n).(*hamt.HashMapRoot)
		mirroredMhIter = provider.HamtMultihashIterator(root, te.mirrorSyncLs)
		require.NoError(t, err)
	default:
		t.Fatal("unknown entries remap prototype", te.mirror.EntriesRemapPrototype())
	}

	var gotMhs []multihash.Multihash
	for {
		next, err := mirroredMhIter.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		gotMhs = append(gotMhs, next)
	}
	require.ElementsMatch(t, wantMhs, gotMhs)
}

func (te *testEnv) syncFromMirrorRecursively(ctx context.Context, c cid.Cid) error {
	if exists, err := te.mirrorSyncLsStore.Has(ctx, cidlink.Link{Cid: c}.Binary()); err != nil {
		return err
	} else if exists {
		return nil
	}
	if te.mirrorSyncer == nil {
		return errors.New("start mirror first")
	}
	return te.mirrorSyncer.Sync(ctx, c, selectorparse.CommonSelector_MatchPoint)
}

func (te *testEnv) syncMirrorAd(ctx context.Context, adCid cid.Cid) (*schema.Advertisement, error) {
	if err := te.syncFromMirrorRecursively(ctx, adCid); err != nil {
		return nil, err
	}
	n, err := te.mirrorSyncLs.Load(ipld.LinkContext{Ctx: ctx}, cidlink.Link{Cid: adCid}, schema.AdvertisementPrototype)
	if err != nil {
		return nil, err
	}
	return schema.UnwrapAdvertisement(n)
}
