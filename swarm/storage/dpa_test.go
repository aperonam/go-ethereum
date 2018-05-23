// Copyright 2016 The go-ethereum Authors
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

package storage

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/log"
)

const testDataSize = 0x1000000

func TestDPArandom(t *testing.T) {
	testDpaRandom(false, t)
	testDpaRandom(true, t)
}

func testDpaRandom(toEncrypt bool, t *testing.T) {
	tdb, err := newTestDbStore(false, false)
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	defer tdb.close()
	db := tdb.LDBStore
	db.setCapacity(50000)
	memStore := NewMemStore(NewDefaultStoreParams(), db)
	localStore := &LocalStore{
		memStore: memStore,
		DbStore:  db,
	}

	dpa := NewDPAAPI(NewFakeDPA(localStore), NewDPAParams())
	defer os.RemoveAll("/tmp/bzz")

	reader, slice := GenerateRandomData(testDataSize)
	ctx, cancel := context.WithTimeout(context.Background(), splitTimeout)
	defer cancel()
	key, wait, err := dpa.Store(ctx, reader, testDataSize, toEncrypt)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}
	log.Warn("store called")
	err = wait(ctx)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}
	log.Warn("store complete", "address", key.Hex())

	resultReader, isEncrypted := dpa.Retrieve(key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	resultSlice := make([]byte, len(slice))
	n, err := resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error.")
	}
	ioutil.WriteFile("/tmp/slice.bzz.16M", slice, 0666)
	ioutil.WriteFile("/tmp/result.bzz.16M", resultSlice, 0666)
	localStore.memStore = NewMemStore(NewDefaultStoreParams(), db)
	resultReader, isEncrypted = dpa.Retrieve(key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	for i := range resultSlice {
		resultSlice[i] = 0
	}
	n, err = resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error after removing memStore: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error after removing memStore got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error after removing memStore.")
	}
}

func TestDPA_capacity(t *testing.T) {
	testDPA_capacity(false, t)
	testDPA_capacity(true, t)
}

func testDPA_capacity(toEncrypt bool, t *testing.T) {
	tdb, err := newTestDbStore(false, false)
	if err != nil {
		t.Fatalf("init dbStore failed: %v", err)
	}
	defer tdb.close()
	db := tdb.LDBStore
	memStore := NewMemStore(NewDefaultStoreParams(), db)
	localStore := &LocalStore{
		memStore: memStore,
		DbStore:  db,
	}
	dpa := NewDPAAPI(NewFakeDPA(localStore), NewDPAParams())
	reader, slice := GenerateRandomData(testDataSize)
	ctx, cancel := context.WithTimeout(context.Background(), splitTimeout)
	defer cancel()
	key, wait, err := dpa.Store(ctx, reader, testDataSize, toEncrypt)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}
	err = wait(ctx)
	if err != nil {
		t.Fatalf("Store error: %v", err)
	}
	resultReader, isEncrypted := dpa.Retrieve(key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	resultSlice := make([]byte, len(slice))
	n, err := resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error.")
	}
	// Clear memStore
	memStore.setCapacity(0)
	// check whether it is, indeed, empty
	dpa.DPA = NewFakeDPA(&chunkMemStore{memStore})
	resultReader, isEncrypted = dpa.Retrieve(key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	if _, err = resultReader.ReadAt(resultSlice, 0); err == nil {
		t.Fatalf("Was able to read %d bytes from an empty memStore.", len(slice))
	}
	// check how it works with localStore
	dpa.DPA = NewFakeDPA(localStore)
	//	localStore.dbStore.setCapacity(0)
	resultReader, isEncrypted = dpa.Retrieve(key)
	if isEncrypted != toEncrypt {
		t.Fatalf("isEncrypted expected %v got %v", toEncrypt, isEncrypted)
	}
	for i := range resultSlice {
		resultSlice[i] = 0
	}
	n, err = resultReader.ReadAt(resultSlice, 0)
	if err != io.EOF {
		t.Fatalf("Retrieve error after clearing memStore: %v", err)
	}
	if n != len(slice) {
		t.Fatalf("Slice size error after clearing memStore got %d, expected %d.", n, len(slice))
	}
	if !bytes.Equal(slice, resultSlice) {
		t.Fatalf("Comparison error after clearing memStore.")
	}
}
