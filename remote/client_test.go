/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package remote_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/little-cui/etcdadpt"
	"github.com/little-cui/etcdadpt/remote"
	"github.com/stretchr/testify/assert"
)

const (
	dialTimeout    = 500 * time.Millisecond
	requestTimeout = time.Second
)

var (
	endpoint = "http://127.0.0.1:2379"
)

func TestInitCluster(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterAddresses = "127.0.0.1:2379"
		assert.Equal(t, "127.0.0.1:2379", strings.Join(etcdadpt.GetClusterURL(cfg.ClusterName, cfg.ClusterAddresses, cfg.ManagerAddress), ","))
	})
	t.Run("not normal2", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterAddresses = "127.0.0.1:2379,127.0.0.2:2379"
		assert.Equal(t, "127.0.0.1:2379,127.0.0.2:2379", strings.Join(etcdadpt.GetClusterURL(cfg.ClusterName, cfg.ClusterAddresses, cfg.ManagerAddress), ","))
	})
	t.Run("1 cluster, should return ok", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterName = "sc-0"
		cfg.ClusterAddresses = "sc-0=127.0.0.1:2379,127.0.0.2:2379"
		clusters := etcdadpt.ParseClusters(cfg.ClusterName, cfg.ClusterAddresses, cfg.ManagerAddress)
		assert.Equal(t, "127.0.0.1:2379,127.0.0.2:2379", strings.Join(clusters[cfg.ClusterName], ","))
		assert.Equal(t, "127.0.0.1:2379,127.0.0.2:2379", strings.Join(clusters["sc-0"], ","))
	})
	t.Run("2 cluster, should return ok", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterName = "sc-0"
		cfg.ClusterAddresses = "sc-1=127.0.0.1:2379,127.0.0.2:2379,sc-2=127.0.0.3:2379"
		clusters := etcdadpt.ParseClusters(cfg.ClusterName, cfg.ClusterAddresses, cfg.ManagerAddress)
		assert.Equal(t, "", strings.Join(clusters[cfg.ClusterName], ","))
		assert.Equal(t, "127.0.0.1:2379,127.0.0.2:2379", strings.Join(clusters["sc-1"], ","))
		assert.Equal(t, "127.0.0.3:2379", strings.Join(clusters["sc-2"], ","))
	})
	t.Run("2 cluster, should return ok", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterName = "sc-0"
		cfg.ClusterAddresses = "sc-0=127.0.0.1:2379,sc-1=127.0.0.3:2379,127.0.0.4:2379"
		clusters := etcdadpt.ParseClusters(cfg.ClusterName, cfg.ClusterAddresses, cfg.ManagerAddress)
		assert.Equal(t, "127.0.0.1:2379", strings.Join(clusters[cfg.ClusterName], ","))
		assert.Equal(t, "127.0.0.3:2379,127.0.0.4:2379", strings.Join(clusters["sc-1"], ","))
	})
	t.Run("have manager address, should return ok", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterName = "sc-0"
		cfg.ManagerAddress = "127.0.0.1:2379,127.0.0.2:2379"
		cfg.ClusterAddresses = "sc-0=127.0.0.1:30100,sc-1=127.0.0.2:30100"
		clusters := etcdadpt.ParseClusters(cfg.ClusterName, cfg.ClusterAddresses, cfg.ManagerAddress)
		assert.Equal(t, "127.0.0.1:2379,127.0.0.2:2379", strings.Join(clusters[cfg.ClusterName], ","))
		assert.Equal(t, "127.0.0.2:30100", strings.Join(clusters["sc-1"], ","))
	})
}

func TestEtcdClient(t *testing.T) {
	t.Run("new client, should return ok", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterAddresses = endpoint
		cfg.DialTimeout = dialTimeout

		inst := remote.NewClient(cfg)
		defer inst.Close()

		select {
		case <-inst.Ready():
		default:
			err := <-inst.Err()
			assert.NoError(t, err)
		}
	})

	t.Run("new client with wrong endpoint, should return err", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterAddresses = "x"
		cfg.DialTimeout = dialTimeout
		inst := remote.NewClient(cfg)
		assert.NotNil(t, inst)
		defer inst.Close()

		select {
		case err := <-inst.(*remote.Client).Err():
			assert.Error(t, err)
		default:
			assert.Fail(t, "should return err")
		}
	})

	t.Run("client do, should return ok", func(t *testing.T) {
		var cfg etcdadpt.Config
		cfg.ClusterAddresses = endpoint
		cfg.DialTimeout = dialTimeout
		cfg.RequestTimeOut = requestTimeout
		inst := remote.NewClient(cfg)
		defer inst.Close()
		// put
		resp, err := inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrValue("b"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/b"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(1), resp.Count)
		assert.Equal(t, "/test_range/b", string(resp.Kvs[0].Key))
		assert.Equal(t, "b", string(resp.Kvs[0].Value))

		resp, err = inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithStrValue("a"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithKeyOnly())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(1), resp.Count)
		assert.Equal(t, "/test_range/a", string(resp.Kvs[0].Key))
		assert.Nil(t, resp.Kvs[0].Value)

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithCountOnly())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(1), resp.Count)
		assert.Nil(t, resp.Kvs)

		resp, err = inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_range/c"),
			etcdadpt.WithStrValue("c"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_range/d"),
			etcdadpt.WithStrValue("d"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_range/dd"),
			etcdadpt.WithStrValue("dd"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		// get prefix
		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/d"),
			etcdadpt.WithPrefix())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(2), resp.Count)
		assert.Equal(t, "/test_range/d", string(resp.Kvs[0].Key))
		assert.Equal(t, "d", string(resp.Kvs[0].Value))
		assert.Equal(t, "/test_range/dd", string(resp.Kvs[1].Key))
		assert.Equal(t, "dd", string(resp.Kvs[1].Value))

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/d"),
			etcdadpt.WithPrefix(), etcdadpt.WithKeyOnly())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(2), resp.Count)
		assert.Equal(t, "/test_range/d", string(resp.Kvs[0].Key))
		assert.Nil(t, resp.Kvs[0].Value)
		assert.Equal(t, "/test_range/dd", string(resp.Kvs[1].Key))
		assert.Nil(t, resp.Kvs[1].Value)

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/d"),
			etcdadpt.WithPrefix(), etcdadpt.WithCountOnly())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(2), resp.Count)
		assert.Nil(t, resp.Kvs)

		// get range
		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrEndKey("/test_range/dd")) // [b, dd) !!!
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(3), resp.Count)
		assert.Equal(t, "/test_range/b", string(resp.Kvs[0].Key))
		assert.Equal(t, "b", string(resp.Kvs[0].Value))
		assert.Equal(t, "/test_range/c", string(resp.Kvs[1].Key))
		assert.Equal(t, "c", string(resp.Kvs[1].Value))
		assert.Equal(t, "/test_range/d", string(resp.Kvs[2].Key))
		assert.Equal(t, "d", string(resp.Kvs[2].Value))

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrEndKey("/test_range/dd"), etcdadpt.WithKeyOnly()) // [b, dd) !!!
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(3), resp.Count)
		assert.Equal(t, "/test_range/b", string(resp.Kvs[0].Key))
		assert.Nil(t, resp.Kvs[0].Value)
		assert.Equal(t, "/test_range/c", string(resp.Kvs[1].Key))
		assert.Nil(t, resp.Kvs[1].Value)
		assert.Equal(t, "/test_range/d", string(resp.Kvs[2].Key))
		assert.Nil(t, resp.Kvs[2].Value)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrEndKey("/test_range/dd"), etcdadpt.WithCountOnly()) // [b, dd) !!!
		if err != nil || !resp.Succeeded || resp.Count != 3 || resp.Kvs != nil {
			t.Fatalf("TestEtcdClient_Do failed, %#v", err)
		}
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(3), resp.Count)
		assert.Nil(t, resp.Kvs)

		// get prefix paging
		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/"), etcdadpt.WithPrefix(),
			etcdadpt.WithOffset(2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(5), resp.Count)
		assert.Equal(t, 2, len(resp.Kvs))
		assert.Equal(t, "/test_range/c", string(resp.Kvs[0].Key))
		assert.Equal(t, "c", string(resp.Kvs[0].Value))
		assert.Equal(t, "/test_range/d", string(resp.Kvs[1].Key))
		assert.Equal(t, "d", string(resp.Kvs[1].Value))

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/"), etcdadpt.WithPrefix(), etcdadpt.WithKeyOnly(),
			etcdadpt.WithOffset(4), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(5), resp.Count)
		assert.Equal(t, 1, len(resp.Kvs))
		assert.Equal(t, "/test_range/dd", string(resp.Kvs[0].Key))
		assert.Nil(t, resp.Kvs[0].Value)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/d"), etcdadpt.WithPrefix(), etcdadpt.WithKeyOnly(),
			etcdadpt.WithOffset(0), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(2), resp.Count)
		assert.Equal(t, 2, len(resp.Kvs))
		assert.Equal(t, "/test_range/d", string(resp.Kvs[0].Key))
		assert.Nil(t, resp.Kvs[0].Value)
		assert.Equal(t, "/test_range/dd", string(resp.Kvs[1].Key))
		assert.Nil(t, resp.Kvs[1].Value)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/"), etcdadpt.WithPrefix(), etcdadpt.WithCountOnly(),
			etcdadpt.WithOffset(2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(5), resp.Count)
		assert.Nil(t, resp.Kvs)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/"), etcdadpt.WithPrefix(),
			etcdadpt.WithOffset(6), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(5), resp.Count)
		assert.Equal(t, 0, len(resp.Kvs))

		// if offset < -1, just paging by limit
		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/"), etcdadpt.WithPrefix(),
			etcdadpt.WithOffset(-2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(5), resp.Count)
		assert.Equal(t, 5, len(resp.Kvs))

		// get range paging
		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrEndKey("/test_range/dd"),
			etcdadpt.WithOffset(2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(3), resp.Count)
		assert.Equal(t, 1, len(resp.Kvs))

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithStrEndKey("/test_range/dd"),
			etcdadpt.WithOffset(2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(4), resp.Count)
		assert.Equal(t, 2, len(resp.Kvs))
		assert.Equal(t, "/test_range/c", string(resp.Kvs[0].Key))
		assert.Equal(t, "/test_range/d", string(resp.Kvs[1].Key))

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithStrEndKey("/test_range/dd"), etcdadpt.WithKeyOnly(),
			etcdadpt.WithOffset(2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(4), resp.Count)
		assert.Equal(t, 2, len(resp.Kvs))
		assert.Equal(t, "/test_range/c", string(resp.Kvs[0].Key))
		assert.Nil(t, resp.Kvs[0].Value)
		assert.Equal(t, "/test_range/d", string(resp.Kvs[1].Key))
		assert.Nil(t, resp.Kvs[1].Value)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithStrEndKey("/test_range/dd"), etcdadpt.WithCountOnly(),
			etcdadpt.WithOffset(2), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(4), resp.Count)
		assert.Nil(t, resp.Kvs)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrEndKey("/test_range/dd"),
			etcdadpt.WithOffset(5), etcdadpt.WithLimit(2))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(3), resp.Count)
		assert.Equal(t, 0, len(resp.Kvs))

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_range/a"),
			etcdadpt.WithStrEndKey("/test_range/dd"),
			etcdadpt.WithOffset(4), etcdadpt.WithLimit(2))
		if err != nil || !resp.Succeeded || resp.Count != 4 || len(resp.Kvs) != 0 {
			t.Fatalf("TestEtcdClient_Do failed, %#v", err)
		}
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(4), resp.Count)
		assert.Equal(t, 0, len(resp.Kvs))

		// delete range
		resp, err = inst.Do(context.Background(), etcdadpt.DEL,
			etcdadpt.WithStrKey("/test_range/b"),
			etcdadpt.WithStrEndKey("/test_range/dd")) // [b, d) !!!
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/"),
			etcdadpt.WithPrefix())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, 2, len(resp.Kvs))
		assert.Equal(t, "/test_range/dd", string(resp.Kvs[1].Key))

		// delete prefix
		resp, err = inst.Do(context.Background(), etcdadpt.DEL, etcdadpt.WithStrKey("/test_range/"),
			etcdadpt.WithPrefix())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_range/"),
			etcdadpt.WithPrefix())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(0), resp.Count)

		// large data
		var wg sync.WaitGroup
		for i := 0; i < etcdadpt.DefaultPageCount+1; i++ {
			wg.Add(1)
			v := strconv.Itoa(i)
			go func() {
				defer wg.Done()
				resp, err = inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_page/"+v),
					etcdadpt.WithStrValue(v))
				assert.NoError(t, err)
				assert.True(t, resp.Succeeded)
			}()
		}
		wg.Wait()
		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_page/"),
			etcdadpt.WithStrEndKey("/test_page/9999"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(etcdadpt.DefaultPageCount+1), resp.Count)
		assert.Equal(t, etcdadpt.DefaultPageCount+1, len(resp.Kvs))

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_page/"), etcdadpt.WithPrefix(), etcdadpt.WithDescendOrder())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(etcdadpt.DefaultPageCount+1), resp.Count)
		assert.Equal(t, etcdadpt.DefaultPageCount+1, len(resp.Kvs))
		assert.Equal(t, "/test_page/999", string(resp.Kvs[0].Key))

		// delete range
		resp, err = inst.Do(context.Background(), etcdadpt.DEL,
			etcdadpt.WithStrKey("/test_page/"),
			etcdadpt.WithStrEndKey("/test_page/9999"))
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)

		resp, err = inst.Do(context.Background(), etcdadpt.GET,
			etcdadpt.WithStrKey("/test_page/"), etcdadpt.WithPrefix())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
		assert.Equal(t, int64(0), resp.Count)
	})
}

func TestEtcdClient_Compact(t *testing.T) {
	var cfg etcdadpt.Config
	cfg.ClusterAddresses = endpoint
	cfg.DialTimeout = dialTimeout
	cfg.RequestTimeOut = requestTimeout

	inst := remote.NewClient(cfg)
	defer inst.Close()

	err := inst.Compact(context.Background(), 0)
	assert.NoError(t, err)
	err = inst.Compact(context.Background(), 0)
	assert.Error(t, err)
}

func TestEtcdClient_Txn(t *testing.T) {
	var cfg etcdadpt.Config
	cfg.ClusterAddresses = endpoint
	cfg.DialTimeout = dialTimeout
	cfg.RequestTimeOut = requestTimeout

	inst := remote.NewClient(cfg)
	defer inst.Close()

	resp, err := inst.Txn(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, resp)

	resp, err = inst.Txn(context.Background(), []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/a"), Value: []byte("a")},
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/b"), Value: []byte("b")},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Succeeded)

	resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_txn/"),
		etcdadpt.WithPrefix(), etcdadpt.WithCountOnly())
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Succeeded)
	assert.Equal(t, int64(2), resp.Count)

	resp, err = inst.TxnWithCmp(context.Background(), []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/a"), Value: []byte("a")},
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/b"), Value: []byte("b")},
	}, []etcdadpt.CmpOptions{
		{[]byte("/test_txn/a"), etcdadpt.CmpValue, etcdadpt.CmpEqual, "a"},
	}, []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/c"), Value: []byte("c")},
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/d"), Value: []byte("d")},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Succeeded)

	// case: range request
	resp, err = inst.TxnWithCmp(context.Background(), nil, []etcdadpt.CmpOptions{
		{[]byte("/test_txn/c"), etcdadpt.CmpValue, etcdadpt.CmpEqual, "c"},
	}, []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionGet, Key: []byte("/test_txn/a")},
		{Action: etcdadpt.ActionGet, Key: []byte("/test_txn/"), Prefix: true},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Succeeded)
	assert.Equal(t, int64(3), resp.Count)

	// case: test key not exist
	resp, err = inst.TxnWithCmp(context.Background(), []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/a"), Value: []byte("a")},
		{Action: etcdadpt.ActionPut, Key: []byte("/test_txn/b"), Value: []byte("b")},
	}, []etcdadpt.CmpOptions{
		{[]byte("/test_txn/c"), etcdadpt.CmpValue, etcdadpt.CmpEqual, "c"},
	}, []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionDelete, Key: []byte("/test_txn/"), Prefix: true},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Succeeded)

	resp, err = inst.Do(context.Background(), etcdadpt.GET, etcdadpt.WithStrKey("/test_txn/"),
		etcdadpt.WithPrefix(), etcdadpt.WithCountOnly())
	if err != nil || !resp.Succeeded || resp.Count != 0 {
		t.Fatalf("TestEtcdClient_Do failed, %#v", err)
	}
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Succeeded)
	assert.Equal(t, int64(0), resp.Count)
}

func TestEtcdClient_LeaseRenew(t *testing.T) {
	var cfg etcdadpt.Config
	cfg.ClusterAddresses = endpoint
	cfg.DialTimeout = dialTimeout
	cfg.RequestTimeOut = requestTimeout

	inst := remote.NewClient(cfg)
	defer inst.Close()

	id, err := inst.LeaseGrant(context.Background(), -1)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), id)

	id, err = inst.LeaseGrant(context.Background(), 0)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), id)

	id, err = inst.LeaseGrant(context.Background(), 2)
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), id)

	ttl, err := inst.LeaseRenew(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), ttl)

	err = inst.LeaseRevoke(context.Background(), id)
	assert.NoError(t, err)

	ttl, err = inst.LeaseRenew(context.Background(), id)
	assert.Error(t, err)
	assert.Equal(t, int64(0), ttl)
}

func TestEtcdClient_HealthCheck(t *testing.T) {
	var cfg etcdadpt.Config
	cfg.ClusterAddresses = endpoint
	cfg.DialTimeout = dialTimeout
	cfg.RequestTimeOut = requestTimeout
	cfg.AutoSyncInterval = time.Millisecond

	inst := remote.NewClient(cfg).(*remote.Client)
	defer inst.Close()

	t.Run("reopen, should return ok", func(t *testing.T) {
		err := inst.ReOpen()
		assert.NoError(t, err)
		ctx, _ := context.WithTimeout(context.Background(), dialTimeout)
		err = inst.SyncMembers(ctx)
		assert.NoError(t, err)
		inst.Endpoints = []string{"x"}
		err = inst.ReOpen()
		assert.Error(t, err)
	})

	t.Run("before check", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), dialTimeout)
		err := inst.SyncMembers(ctx)
		assert.NoError(t, err)
	})

	t.Run("check health, should return ok", func(t *testing.T) {
		var err error
		inst.Endpoints = []string{endpoint}
		for {
			_, err = inst.Do(context.Background(), etcdadpt.GET,
				etcdadpt.WithStrKey("/test_health/"))
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			} else {
				break
			}
		}
		assert.NoError(t, err)
	})

}

func TestEtcdClient_Watch(t *testing.T) {
	var cfg etcdadpt.Config
	cfg.ClusterAddresses = endpoint
	cfg.DialTimeout = dialTimeout
	cfg.RequestTimeOut = requestTimeout
	cfg.AutoSyncInterval = time.Millisecond

	inst := remote.NewClient(cfg)
	defer inst.Close()

	defer func() {
		resp, err := inst.Do(context.Background(), etcdadpt.DEL, etcdadpt.WithStrKey("/test_watch/"),
			etcdadpt.WithPrefix())
		assert.NoError(t, err)
		assert.True(t, resp.Succeeded)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := inst.Watch(ctx, etcdadpt.WithStrKey("/test_watch/a"))
	assert.NoError(t, err)

	ch := make(chan struct{})
	go func() {
		defer func() { ch <- struct{}{} }()
		err = inst.Watch(context.Background(), etcdadpt.WithStrKey("/test_watch/a"),
			etcdadpt.WithWatchCallback(func(message string, evt *etcdadpt.Response) error {
				assert.Equal(t, int64(1), evt.Count)
				assert.Equal(t, 1, len(evt.Kvs))
				assert.Equal(t, etcdadpt.ActionPut, evt.Action)
				assert.Equal(t, "/test_watch/a", string(evt.Kvs[0].Key))
				assert.Equal(t, "a", string(evt.Kvs[0].Value))
				return fmt.Errorf("error")
			}))
		assert.Equal(t, "error", err.Error())
	}()

	<-time.After(500 * time.Millisecond)
	resp, err := inst.Do(context.Background(), etcdadpt.PUT, etcdadpt.WithStrKey("/test_watch/a"),
		etcdadpt.WithStrValue("a"))
	assert.NoError(t, err)
	assert.True(t, resp.Succeeded)
	<-ch

	go func() {
		defer func() { ch <- struct{}{} }()
		err = inst.Watch(context.Background(), etcdadpt.WithStrKey("/test_watch/"),
			etcdadpt.WithPrefix(),
			etcdadpt.WithWatchCallback(func(message string, evt *etcdadpt.Response) error {
				equalA := evt.Action == etcdadpt.ActionPut && string(evt.Kvs[0].Key) == "/test_watch/a" && string(evt.Kvs[0].Value) == "a"
				equalB := evt.Action == etcdadpt.ActionPut && string(evt.Kvs[1].Key) == "/test_watch/b" && string(evt.Kvs[0].Value) == "b"
				assert.Equal(t, int64(2), evt.Count)
				assert.Equal(t, 2, len(evt.Kvs))
				assert.True(t, equalA || equalB)
				return fmt.Errorf("error")
			}))
		assert.Equal(t, "error", err.Error())
	}()

	<-time.After(500 * time.Millisecond)
	resp, err = inst.Txn(context.Background(), []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionPut, Key: []byte("/test_watch/a"), Value: []byte("a")},
		{Action: etcdadpt.ActionPut, Key: []byte("/test_watch/b"), Value: []byte("b")},
	})
	assert.NoError(t, err)
	assert.True(t, resp.Succeeded)
	<-ch

	// diff action type will be split
	go func() {
		defer func() { ch <- struct{}{} }()
		var times = 3
		err = inst.Watch(context.Background(), etcdadpt.WithStrKey("/test_watch/"),
			etcdadpt.WithPrefix(),
			etcdadpt.WithWatchCallback(func(message string, evt *etcdadpt.Response) error {
				equalA := evt.Action == etcdadpt.ActionDelete && string(evt.Kvs[0].Key) == "/test_watch/a" && evt.Kvs[0].Value == nil
				equalB := evt.Action == etcdadpt.ActionPut && string(evt.Kvs[0].Key) == "/test_watch/b" && string(evt.Kvs[0].Value) == "b"
				equalC := evt.Action == etcdadpt.ActionPut && string(evt.Kvs[0].Key) == "/test_watch/c" && string(evt.Kvs[0].Value) == "c"
				assert.Equal(t, int64(1), evt.Count)
				assert.Equal(t, 1, len(evt.Kvs))
				assert.True(t, equalA || equalB || equalC)

				times--
				if times == 0 {
					return fmt.Errorf("error")
				}
				return nil
			}))
		assert.Equal(t, "error", err.Error())
	}()

	<-time.After(500 * time.Millisecond)
	resp, err = inst.Txn(context.Background(), []etcdadpt.OpOptions{
		{Action: etcdadpt.ActionPut, Key: []byte("/test_watch/c"), Value: []byte("c")},
		{Action: etcdadpt.ActionDelete, Key: []byte("/test_watch/a"), Value: []byte("a")},
		{Action: etcdadpt.ActionPut, Key: []byte("/test_watch/b"), Value: []byte("b")},
	})
	assert.NoError(t, err)
	assert.True(t, resp.Succeeded)
	<-ch

	// watch with rev
	resp, err = inst.Do(context.Background(), etcdadpt.DEL, etcdadpt.WithStrKey("/test_watch/c"),
		etcdadpt.WithStrValue("a"))
	assert.NoError(t, err)
	assert.True(t, resp.Succeeded)

	rev := resp.Revision
	go func() {
		defer func() { ch <- struct{}{} }()
		err = inst.Watch(context.Background(), etcdadpt.WithStrKey("/test_watch/"),
			etcdadpt.WithPrefix(),
			etcdadpt.WithRev(rev),
			etcdadpt.WithWatchCallback(func(message string, evt *etcdadpt.Response) error {
				assert.Equal(t, int64(1), evt.Count)
				assert.Equal(t, 1, len(evt.Kvs))
				assert.Equal(t, etcdadpt.ActionDelete, evt.Action)
				assert.Equal(t, "/test_watch/c", string(evt.Kvs[0].Key))
				assert.Nil(t, evt.Kvs[0].Value)
				return fmt.Errorf("error")
			}))
		assert.Equal(t, "error", err.Error())
	}()
	<-ch

	// delete with prevKV
	go func() {
		defer func() { ch <- struct{}{} }()
		err = inst.Watch(context.Background(), etcdadpt.WithStrKey("/test_watch/"),
			etcdadpt.WithPrefix(), etcdadpt.WithPrevKv(),
			etcdadpt.WithWatchCallback(func(message string, evt *etcdadpt.Response) error {
				assert.Equal(t, 1, len(evt.Kvs))
				assert.Equal(t, etcdadpt.ActionDelete, evt.Action)
				assert.Equal(t, "/test_watch/b", string(evt.Kvs[0].Key))
				assert.Equal(t, "b", string(evt.Kvs[0].Value))
				return fmt.Errorf("error")
			}))
		assert.Equal(t, "error", err.Error())
	}()

	<-time.After(500 * time.Millisecond)
	resp, err = inst.Do(context.Background(), etcdadpt.DEL, etcdadpt.WithStrKey("/test_watch/b"))
	assert.NoError(t, err)
	assert.True(t, resp.Succeeded)
	<-ch
}

type mockKVForPaging struct {
	rangeCount int
	countResp  *clientv3.GetResponse
	rangeResp1 *clientv3.GetResponse
	rangeResp2 *clientv3.GetResponse
}

func (m *mockKVForPaging) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return nil, nil
}

func (m *mockKVForPaging) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	op := &clientv3.Op{}
	for _, o := range opts {
		o(op)
	}
	if op.IsCountOnly() {
		return m.countResp, nil
	}
	if m.rangeCount == 0 {
		m.rangeCount = 1
		return m.rangeResp1, nil
	}
	return m.rangeResp2, nil
}

func (m *mockKVForPaging) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return nil, nil
}

func (m *mockKVForPaging) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return nil, nil
}

func (m *mockKVForPaging) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (m *mockKVForPaging) Txn(ctx context.Context) clientv3.Txn {
	return nil
}

// test scenario: db data decreases during paging.
func TestEtcdClient_paging(t *testing.T) {
	// key range: [startKey, endKey)
	generateGetResp := func(startKey, endKey int) *clientv3.GetResponse {
		resp := &clientv3.GetResponse{
			Count: int64(endKey - startKey),
			Header: &etcdserverpb.ResponseHeader{
				Revision: 0,
			},
			Kvs: make([]*mvccpb.KeyValue, 0),
		}
		if resp.Count <= 0 {
			return resp
		}
		for i := startKey; i < endKey; i++ {
			kvPart := &mvccpb.KeyValue{
				Key:   []byte(fmt.Sprint(i)),
				Value: []byte(""),
			}
			resp.Kvs = append(resp.Kvs, kvPart)
		}
		return resp
	}

	mockKv := &mockKVForPaging{
		rangeCount: 0,
		// if count only, return 4097 kvs
		countResp: generateGetResp(0, 4097),
		// the first paging request, return 4096 kvs
		rangeResp1: generateGetResp(0, 4096),
		// the second paging request, return 0 kv
		// meaning data decreases during paging
		rangeResp2: generateGetResp(0, 0),
	}
	c := remote.Client{
		Client: &clientv3.Client{
			KV: mockKv,
		},
	}

	op := etcdadpt.OpOptions{
		Offset: -1,
		Limit:  etcdadpt.DefaultPageCount,
	}
	r, err := c.Paging(context.Background(), op)
	if err != nil {
		t.Fatalf("TestEtcdClient_paging failed, %#v", err)
	}
	if len(r.Kvs) <= 0 {
		t.Fatalf("TestEtcdClient_paging failed")
	}
}