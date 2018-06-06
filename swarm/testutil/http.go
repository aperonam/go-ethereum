// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package testutil

import (
	"context"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/metrics/influxdb"
	"github.com/ethereum/go-ethereum/swarm/api"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/swarm/storage/mru"
)

type TestServer interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type fakeBackend struct {
	blocknumber int64
}

func (f *fakeBackend) HeaderByNumber(context context.Context, _ string, bigblock *big.Int) (*types.Header, error) {
	f.blocknumber++
	biggie := big.NewInt(f.blocknumber)
	return &types.Header{
		Number: biggie,
	}, nil
}

func NewTestSwarmServer(t *testing.T, serverFunc func(*api.Api) TestServer) *TestSwarmServer {
	dir, err := ioutil.TempDir("", "swarm-storage-test")
	if err != nil {
		t.Fatal(err)
	}
	storeparams := storage.NewDefaultLocalStoreParams()
	storeparams.DbCapacity = 5000000
	storeparams.CacheCapacity = 5000
	storeparams.Init(dir)
	localStore, err := storage.NewLocalStore(storeparams, nil)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	fileStore := storage.NewFileStore(localStore, storage.NewFileStoreParams())

	// mutable resources test setup
	resourceDir, err := ioutil.TempDir("", "swarm-resource-test")
	if err != nil {
		t.Fatal(err)
	}

	// signer
	privKeyBytes := [32]byte{}
	privKeyBytes[0] = 0x2a
	privKey, err := crypto.ToECDSA(privKeyBytes[:])
	if err != nil {
		t.Fatal(err)
	}
	signer := mru.NewGenericSigner(privKey)
	rhparams := &mru.HandlerParams{
		QueryMaxPeriods: &mru.LookupParams{},
		Signer:          signer,
		HeaderGetter: &fakeBackend{
			blocknumber: 42,
		},
	}
	rh, err := mru.NewTestHandler(resourceDir, rhparams)
	if err != nil {
		t.Fatal(err)
	}

	a := api.NewApi(fileStore, nil, rh.Handler)
	srv := httptest.NewServer(serverFunc(a))
	return &TestSwarmServer{
		Server:    srv,
		FileStore: fileStore,
		dir:       dir,
		Hasher:    storage.MakeHashFunc(storage.DefaultHash)(),
		cleanup: func() {
			srv.Close()
			rh.Close()
			os.RemoveAll(dir)
			os.RemoveAll(resourceDir)
		},
	}
}

type TestSwarmServer struct {
	*httptest.Server
	Hasher    storage.SwarmHash
	FileStore *storage.FileStore
	dir       string
	cleanup   func()
}

func (t *TestSwarmServer) Close() {
	t.cleanup()
}

// EnableMetrics is starting InfluxDB reporter so that we collect stats when running tests locally
func EnableMetrics() {
	metrics.Enabled = true
	go influxdb.InfluxDBWithTags(metrics.DefaultRegistry, 1*time.Second, "http://localhost:8086", "metrics", "admin", "admin", "swarm.", map[string]string{
		"host": "test",
	})
}
