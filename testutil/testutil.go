package testutil

import (
	"context"
	crand "crypto/rand"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	dtimpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnetwork "github.com/filecoin-project/go-data-transfer/network"
	gstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	"github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/blockstore"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func NewID(t *testing.T) peer.ID {
	_, pub, err := crypto.GenerateEd25519Key(crand.Reader)
	require.NoError(t, err)

	id, err := peer.IDFromPublicKey(pub)
	require.NoError(t, err)

	return id
}

func RandomCids(t testing.TB, rng *rand.Rand, n int) []cid.Cid {
	prefix := schema.Linkproto.Prefix

	cids := make([]cid.Cid, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 10*n)
		rng.Read(b)
		c, err := prefix.Sum(b)
		require.NoError(t, err)
		cids[i] = c
	}
	return cids
}

func RandomMultihashes(t testing.TB, rng *rand.Rand, n int) []multihash.Multihash {
	prefix := schema.Linkproto.Prefix

	mhashes := make([]multihash.Multihash, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 10*n)
		rng.Read(b)
		c, err := prefix.Sum(b)
		require.NoError(t, err)
		mhashes[i] = c.Hash()
	}
	return mhashes
}

// ThisDir gets the current directory of the source file its called in
func ThisDir(t *testing.T) string {
	_, fname, _, ok := runtime.Caller(1)
	require.True(t, ok)
	return filepath.Dir(fname)
}

// RoBlockstore is just the needed interface for GetBstoreLen
type RoBlockstore interface {
	AllKeysChan(ctx context.Context) (<-chan cid.Cid, error)
}

// GetBstoreLen gets the total CID cound in a blockstore
func GetBstoreLen(ctx context.Context, t *testing.T, bs RoBlockstore) int {
	ch, err := bs.AllKeysChan(ctx)
	require.NoError(t, err)
	var len int
	for range ch {
		len++
	}
	return len
}

// OpenSampleCar opens a car file in the testdata directory to a blockstore
func OpenSampleCar(t *testing.T, carFileName string) *blockstore.ReadOnly {
	carFilePath := filepath.Join(ThisDir(t), "../testdata", carFileName)
	rdOnlyBS, err := blockstore.OpenReadOnly(carFilePath, carv2.ZeroLengthSectionAsEOF(true), blockstore.UseWholeCIDs(true))
	require.NoError(t, err)
	return rdOnlyBS
}

// SetupDataTransferOnHost generates a data transfer instance for the given libp2p host
func SetupDataTransferOnHost(t *testing.T, h host.Host, store datastore.Batching, lsys ipld.LinkSystem) datatransfer.Manager {
	gsnet := gsnet.NewFromLibp2pHost(h)
	dtNet := dtnetwork.NewFromLibp2pHost(h)
	gs := gsimpl.New(context.Background(), gsnet, lsys)
	tp := gstransport.NewTransport(h.ID(), gs)
	dt, err := dtimpl.NewDataTransfer(store, dtNet, tp)
	require.NoError(t, err)
	ready := make(chan error, 1)
	dt.OnReady(func(err error) {
		ready <- err
	})
	require.NoError(t, dt.Start(context.Background()))
	timer := time.NewTimer(500 * time.Millisecond)
	select {
	case readyErr := <-ready:
		require.NoError(t, readyErr)
	case <-timer.C:
		require.FailNow(t, "data transfer did not initialize")
	}
	return dt
}

// CopyDir copies a whole directory recursively
func CopyDir(t *testing.T, src string, dst string) {
	srcinfo, err := os.Stat(src)
	require.NoError(t, err)
	err = os.MkdirAll(dst, srcinfo.Mode())
	require.NoError(t, err)
	fds, err := ioutil.ReadDir(src)
	require.NoError(t, err)

	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			CopyDir(t, srcfp, dstfp)
		} else {
			CopyFile(t, srcfp, dstfp)
		}
	}
}

// CopyFile copies a single file from src to dst
func CopyFile(t *testing.T, src, dst string) {
	srcfd, err := os.Open(src)
	require.NoError(t, err)
	defer srcfd.Close()

	dstfd, err := os.Create(dst)
	require.NoError(t, err)
	defer dstfd.Close()

	_, err = io.Copy(dstfd, srcfd)
	require.NoError(t, err)
}
