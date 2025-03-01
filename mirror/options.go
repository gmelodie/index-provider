package mirror

import (
	"time"

	"github.com/filecoin-project/index-provider/engine/chunker"
	stischema "github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	hamt "github.com/ipld/go-ipld-adl-hamt"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/multiformats/go-multicodec"
)

type (
	Option  func(*options) error
	options struct {
		h                           host.Host
		ds                          datastore.Batching
		ticker                      *time.Ticker
		initAdRecurLimit            selector.RecursionLimit
		entriesRecurLimit           selector.RecursionLimit
		chunkerFunc                 chunker.NewChunkerFunc
		chunkCacheCap               int
		chunkCachePurge             bool
		topic                       string
		skipRemapOnEntriesTypeMatch bool
		entriesRemapPrototype       schema.TypedPrototype
		alwaysReSignAds             bool
	}
)

// TODO: add options to restructure advertisements.
//       nft.storage advertisement chain is a good usecase, where remapping entries to say HAMT
//       probably won't make much difference. But combining ads to make a shorter chain will most
//       likely improve end-to-end ingestion latency.

func newOptions(o ...Option) (*options, error) {
	opts := options{
		ticker:            time.NewTicker(10 * time.Minute),
		initAdRecurLimit:  selector.RecursionLimitNone(),
		entriesRecurLimit: selector.RecursionLimitNone(),
		chunkCacheCap:     1024,
		chunkCachePurge:   false,
		topic:             "/indexer/ingest/mainnet",
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	if opts.h == nil {
		var err error
		if opts.h, err = libp2p.New(); err != nil {
			return nil, err
		}
	}
	if opts.ds == nil {
		opts.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	}
	return &opts, nil
}

func (o *options) remapEntriesEnabled() bool {
	// Use whether the chunker func is set or not as a flag to decide if entries should be remapped.
	return o.chunkerFunc != nil
}

// WithDatastore specifies the datastore used by the mirror to persist mirrored advertisements,
// their entries and other internal data.
// Defaults to an ephemeral in-memory datastore.
func WithDatastore(ds datastore.Batching) Option {
	return func(o *options) error {
		o.ds = ds
		return nil
	}
}

// WithHost specifies the libp2p host the mirror should be exposed on.
// If unspecified a host with default options and random identity is used.
func WithHost(h host.Host) Option {
	return func(o *options) error {
		o.h = h
		return nil
	}
}

// WithEntryChunkRemapper remaps the entries from the original provider into schema.EntryChunkPrototype
// structure with the given chunk size.
// If unset, the original structure is mirrored without change.
//
// See: WithSkipRemapOnEntriesTypeMatch, WithHamtRemapper.
func WithEntryChunkRemapper(chunkSize int) Option {
	return func(o *options) error {
		o.entriesRemapPrototype = stischema.EntryChunkPrototype
		o.chunkerFunc = chunker.NewChainChunkerFunc(chunkSize)
		return nil
	}
}

// WithHamtRemapper remaps the entries from the original provider into hamt.HashMapRootPrototype
// structure with the given bit-width and bucket size.
// If unset, the original structure is mirrored without change.
//
// See: WithSkipRemapOnEntriesTypeMatch, WithEntryChunkRemapper.
func WithHamtRemapper(hashAlg multicodec.Code, bitwidth, bucketSize int) Option {
	return func(o *options) error {
		o.entriesRemapPrototype = hamt.HashMapRootPrototype
		o.chunkerFunc = chunker.NewHamtChunkerFunc(hashAlg, bitwidth, bucketSize)
		return nil
	}
}

// WithSkipRemapOnEntriesTypeMatch specifies weather to skip remapping entries if the original
// structure prototype matches the configured remap option.
// Note that setting this option without setting a remap option has no effect.
//
// See: WithEntryChunkRemapper, WithHamtRemapper.
func WithSkipRemapOnEntriesTypeMatch(s bool) Option {
	return func(o *options) error {
		o.skipRemapOnEntriesTypeMatch = s
		return nil
	}
}

// WithSyncInterval specifies the time interval at which the original provider is checked for new
// advertisements.
// If unset, the default time interval of 10 minutes is used.
func WithSyncInterval(t *time.Ticker) Option {
	return func(o *options) error {
		o.ticker = t
		return nil
	}
}

// WithInitialAdRecursionLimit specifies the recursion limit for the initial sync if no previous
// advertisements are mirrored by the mirror.
// If unset, selector.RecursionLimitNone is used.
func WithInitialAdRecursionLimit(l selector.RecursionLimit) Option {
	return func(o *options) error {
		o.initAdRecurLimit = l
		return nil
	}
}

// WithEntriesRecursionLimit specifies the recursion limit for syncing the advertisement entries.
// If unset, selector.RecursionLimitNone is used.
func WithEntriesRecursionLimit(l selector.RecursionLimit) Option {
	return func(o *options) error {
		o.entriesRecurLimit = l
		return nil
	}
}

// WithRemappedEntriesCacheCapacity sets the LRU cache capacity used to store the remapped
// advertisement entries. The capacity refers to the number of complete entries DAGs cached. The
// actual storage occupied by the cache depends on the shape of the DAGs.
// See: chunker.CachedEntriesChunker.
//
// This option has no effect if no entries remapper option is set.
// Defaults to 1024.
func WithRemappedEntriesCacheCapacity(c int) Option {
	return func(o *options) error {
		o.chunkCacheCap = c
		return nil
	}
}

// WithPurgeCachedEntries specifies whether to delete any cached entries on start-up.
// This option has no effect if no entries remapper option is set.
func WithPurgeCachedEntries(b bool) Option {
	return func(o *options) error {
		o.chunkCachePurge = b
		return nil
	}
}

// WithTopicName specifies the topi name on which the mirrored advertisements are announced.
func WithTopicName(t string) Option {
	return func(o *options) error {
		o.topic = t
		return nil
	}
}

// WithAlwaysReSignAds specifies whether every mirrored ad should be resigned by the mirror identity
// regardless of weather the advertisement content is changed as a result of mirroring or not.
// By default, advertisements are only re-signed if: 1) the link to previous advertisement is not
// changed, and 2) link to entries is not changed.
func WithAlwaysReSignAds(r bool) Option {
	return func(o *options) error {
		o.alwaysReSignAds = r
		return nil
	}
}
