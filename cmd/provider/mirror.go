package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/filecoin-project/index-provider/mirror"
	leveldb "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

var Mirror struct {
	*cli.Command
	flags struct {
		source                      *cli.StringFlag
		syncInterval                *cli.DurationFlag
		identityPath                *cli.PathFlag
		listenAddrs                 *cli.StringSliceFlag
		storePath                   *cli.PathFlag
		initAdRecurLimit            *cli.UintFlag
		entriesRecurLimit           *cli.UintFlag
		remapWithEntryChunkSize     *cli.UintFlag
		remapWithHamtHashFunc       *cli.StringFlag
		remapWithHamtBitWidth       *cli.UintFlag
		remapWithHamtBucketSize     *cli.UintFlag
		topic                       *cli.StringFlag
		skipRemapOnEntriesTypeMatch *cli.BoolFlag
		alwaysReSignAds             *cli.BoolFlag
	}

	source  *peer.AddrInfo
	options []mirror.Option
}

func init() {
	Mirror.flags.source = &cli.StringFlag{
		Name:     "source",
		Usage:    "The addrinfo of the provider to mirror.",
		Required: true,
	}
	Mirror.flags.syncInterval = &cli.DurationFlag{
		Name:        "syncInterval",
		Usage:       "The time interval at which to check the source for new advertisements.",
		DefaultText: "10 minutes",
	}
	Mirror.flags.identityPath = &cli.PathFlag{
		Name:        "identityPath",
		Usage:       "The path to the file containing the marshalled libp2p private key that the mirror should use as its identity.",
		DefaultText: "Randomly generated",
	}
	Mirror.flags.listenAddrs = &cli.StringSliceFlag{
		Name:        "listenAddrs",
		Usage:       "The mirror listen addresses in form of multiaddr.",
		DefaultText: "Local host with a random open port",
	}
	Mirror.flags.storePath = &cli.PathFlag{
		Name:        "storePath",
		Usage:       "The path at which to persist the mirror state.",
		DefaultText: "Ephemeral in-memory storage",
	}
	Mirror.flags.initAdRecurLimit = &cli.UintFlag{
		Name:        "initAdRecurLimit",
		Usage:       "The maximum recursion depth limit of ads to mirror if no previous ads are mirrored.",
		DefaultText: "No limit",
	}
	Mirror.flags.entriesRecurLimit = &cli.UintFlag{
		Name:        "entriesRecurLimit",
		Usage:       "The maximum recursion depth limit of ad entries to mirror.",
		DefaultText: "No limit",
	}
	Mirror.flags.remapWithEntryChunkSize = &cli.UintFlag{
		Name:        "remapWithEntryChunkSize",
		Usage:       "Remaps the advertisement entries to EntryChunk chain with the specified chunk size",
		DefaultText: "No remapping of entries",
	}
	Mirror.flags.remapWithHamtHashFunc = &cli.StringFlag{
		Name: "remapWithHamtHashFunc",
		Usage: "Remaps the advertisement entries to HAMT using the given hash function. " +
			"Only `identity`, `sha2-256` and `murmur3-x64-64` are accepted.",
		DefaultText: "No remapping of entries",
	}
	Mirror.flags.remapWithHamtBitWidth = &cli.UintFlag{
		Name:        "remapWithHamtBitWidth",
		Usage:       "Remaps the advertisement entries to HAMT using the given bit-width.",
		DefaultText: "No remapping of entries",
	}
	Mirror.flags.remapWithHamtBucketSize = &cli.UintFlag{
		Name:        "remapWithHamtBucketSize",
		Usage:       "Remaps the advertisement entries to HAMT using the given bucket size.",
		DefaultText: "No remapping of entries",
	}
	Mirror.flags.topic = &cli.StringFlag{
		Name:        "topic",
		Usage:       "The topic on which the source and mirrored advertisements are announced.",
		DefaultText: "`/indexer/ingest/mainnet`",
	}
	Mirror.flags.skipRemapOnEntriesTypeMatch = &cli.BoolFlag{
		Name:        "skipRemapOnEntriesTypeMatch",
		Usage:       "Whether to skip remapping the entries if the source entries kind matches the required mirrored remap kind.",
		DefaultText: "No remapping of entries",
	}
	Mirror.flags.alwaysReSignAds = &cli.BoolFlag{
		Name:        "alwaysReSignAds",
		Usage:       "Whether to always re-sign advertisements with the mirror's identity.",
		DefaultText: "Ads are only re-singed if changed by the mirror.",
	}
	Mirror.Command = &cli.Command{
		Name:  "mirror",
		Usage: "Mirrors the advertisement chain from an existing index provider.",
		Flags: []cli.Flag{
			Mirror.flags.source,
			Mirror.flags.syncInterval,
			Mirror.flags.identityPath,
			Mirror.flags.listenAddrs,
			Mirror.flags.storePath,
			Mirror.flags.initAdRecurLimit,
			Mirror.flags.entriesRecurLimit,
			Mirror.flags.remapWithEntryChunkSize,
			Mirror.flags.remapWithHamtHashFunc,
			Mirror.flags.remapWithHamtBitWidth,
			Mirror.flags.remapWithHamtBucketSize,
			Mirror.flags.topic,
			Mirror.flags.skipRemapOnEntriesTypeMatch,
			Mirror.flags.alwaysReSignAds,
		},
		Before: beforeMirror,
		Action: doMirror,
	}
}

func beforeMirror(cctx *cli.Context) error {
	var err error
	Mirror.source, err = peer.AddrInfoFromString(Mirror.flags.source.Get(cctx))
	if err != nil {
		return err
	}
	if cctx.IsSet(Mirror.flags.syncInterval.Name) {
		ticker := time.NewTicker(Mirror.flags.syncInterval.Get(cctx))
		Mirror.options = append(Mirror.options, mirror.WithSyncInterval(ticker))
	}
	var hostOpts []libp2p.Option
	if cctx.IsSet(Mirror.flags.identityPath.Name) {
		pkPath := Mirror.flags.identityPath.Get(cctx)
		pkBytes, err := ioutil.ReadFile(pkPath)
		if err != nil {
			return err
		}
		pk, err := crypto.UnmarshalPrivateKey(pkBytes)
		if err != nil {
			return err
		}
		hostOpts = append(hostOpts, libp2p.Identity(pk))
	}
	if cctx.IsSet(Mirror.flags.listenAddrs.Name) {
		listenAddrs := Mirror.flags.listenAddrs.Get(cctx)
		hostOpts = append(hostOpts, libp2p.ListenAddrStrings(listenAddrs...))
	}
	if len(hostOpts) != 0 {
		h, err := libp2p.New(hostOpts...)
		if err != nil {
			return err
		}
		Mirror.options = append(Mirror.options, mirror.WithHost(h))
	}
	if cctx.IsSet(Mirror.flags.storePath.Name) {
		path := Mirror.flags.storePath.Get(cctx)
		ds, err := leveldb.NewDatastore(path, nil)
		if err != nil {
			return err
		}
		Mirror.options = append(Mirror.options, mirror.WithDatastore(ds))
	}
	if cctx.IsSet(Mirror.flags.initAdRecurLimit.Name) {
		limit := selector.RecursionLimitDepth(int64(Mirror.flags.initAdRecurLimit.Get(cctx)))
		Mirror.options = append(Mirror.options, mirror.WithInitialAdRecursionLimit(limit))
	}
	if cctx.IsSet(Mirror.flags.entriesRecurLimit.Name) {
		limit := selector.RecursionLimitDepth(int64(Mirror.flags.entriesRecurLimit.Get(cctx)))
		Mirror.options = append(Mirror.options, mirror.WithEntriesRecursionLimit(limit))
	}

	remapEC := cctx.IsSet(Mirror.flags.remapWithEntryChunkSize.Name)
	remapHamtHF := cctx.IsSet(Mirror.flags.remapWithHamtHashFunc.Name)
	remapHamtBW := cctx.IsSet(Mirror.flags.remapWithHamtBitWidth.Name)
	remapHamtBS := cctx.IsSet(Mirror.flags.remapWithHamtBucketSize.Name)
	switch {
	case remapEC && (remapHamtBS || remapHamtBW || remapHamtHF):
		return errors.New("only one entry remap kind can be specified; both EntryChunk and HAMT flags are set")
	case remapHamtBS != remapHamtBW || remapHamtBS != remapHamtHF:
		return errors.New("to remap entries as HAMT all three of hash function, bit-width and bucket size flags must be set")
	case remapEC:
		chunkSize := Mirror.flags.remapWithEntryChunkSize.Get(cctx)
		Mirror.options = append(Mirror.options, mirror.WithEntryChunkRemapper(int(chunkSize)))
	case remapHamtBS && remapHamtBW && remapHamtHF:
		hf := Mirror.flags.remapWithHamtHashFunc.Get(cctx)
		bw := Mirror.flags.remapWithHamtBitWidth.Get(cctx)
		bs := Mirror.flags.remapWithHamtBucketSize.Get(cctx)

		mhc, ok := multihash.Names[hf]
		if !ok {
			return fmt.Errorf("no multihash code found with name: %s", hf)
		}
		Mirror.options = append(Mirror.options, mirror.WithHamtRemapper(multicodec.Code(mhc), int(bw), int(bs)))
	}
	if cctx.IsSet(Mirror.flags.topic.Name) {
		topic := Mirror.flags.topic.Get(cctx)
		Mirror.options = append(Mirror.options, mirror.WithTopicName(topic))
	}
	if cctx.IsSet(Mirror.flags.skipRemapOnEntriesTypeMatch.Name) {
		s := Mirror.flags.skipRemapOnEntriesTypeMatch.Get(cctx)
		Mirror.options = append(Mirror.options, mirror.WithSkipRemapOnEntriesTypeMatch(s))
	}
	if cctx.IsSet(Mirror.flags.alwaysReSignAds.Name) {
		r := Mirror.flags.alwaysReSignAds.Get(cctx)
		Mirror.options = append(Mirror.options, mirror.WithAlwaysReSignAds(r))
	}
	return nil
}

func doMirror(cctx *cli.Context) error {
	err := logging.SetLogLevel("provider/mirror", "info")
	if err != nil {
		return err
	}
	m, err := mirror.New(cctx.Context, *Mirror.source, Mirror.options...)
	if err != nil {
		return err
	}
	if err = m.Start(); err != nil {
		return err
	}
	<-cctx.Done()
	return m.Shutdown()
}
