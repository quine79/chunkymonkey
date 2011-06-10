package shardserver

import (
	"sync"

	"chunkymonkey/chunkstore"
	"chunkymonkey/entity"
	"chunkymonkey/gamerules"
	"chunkymonkey/stub"
	. "chunkymonkey/types"
)

// LocalShardManager contains all chunk shards and can look them up. It
// implements IShardConnecter and is for use in hosting all shards in the local
// process.
type LocalShardManager struct {
	entityMgr  *entity.EntityManager
	chunkStore chunkstore.IChunkStore
	gameRules  *gamerules.GameRules
	shards     map[uint64]*ChunkShard
	lock       sync.Mutex
}

func NewLocalShardManager(chunkStore chunkstore.IChunkStore, entityMgr *entity.EntityManager, gameRules *gamerules.GameRules) *LocalShardManager {
	return &LocalShardManager{
		entityMgr:  entityMgr,
		chunkStore: chunkStore,
		gameRules:  gameRules,
		shards:     make(map[uint64]*ChunkShard),
	}
}

func (mgr *LocalShardManager) getShard(loc ShardXz) *ChunkShard {
	shardKey := loc.Key()
	if shard, ok := mgr.shards[shardKey]; ok {
		// Shard already exists.
		return shard
	}

	// Create shard.
	shard := NewChunkShard(mgr, loc)
	mgr.shards[shardKey] = shard
	go shard.serve()

	return shard
}

func (mgr *LocalShardManager) ShardConnect(entityId EntityId, player stub.IShardPlayerClient, shardLoc ShardXz) stub.IPlayerShardClient {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	shard := mgr.getShard(shardLoc)
	return newLocalPlayerShardClient(entityId, player, shard)
}

func (mgr *LocalShardManager) ShardToShardConnect(shardLoc ShardXz) stub.IShardShardClient {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	// TODO

	return nil
}

// TODO remove Enqueue* methods

// EnqueueAllChunks runs a given function on all loaded chunks.
func (mgr *LocalShardManager) EnqueueAllChunks(fn func(chunk *Chunk)) {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	for _, shard := range mgr.shards {
		shard.EnqueueAllChunks(fn)
	}
}

// EnqueueOnChunk runs a function on the chunk at the given location. If the
// chunk does not exist, it does nothing.
func (mgr *LocalShardManager) EnqueueOnChunk(loc ChunkXz, fn func(chunk *Chunk)) {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	shard := mgr.getShard(loc.ToShardXz())
	shard.EnqueueOnChunk(loc, fn)
}
