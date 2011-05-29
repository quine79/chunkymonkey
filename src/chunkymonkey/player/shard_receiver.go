package player

import (
	"chunkymonkey/slot"
	. "chunkymonkey/types"
)

// playerShardReceiver receives events from chunk shards and acts upon them. It
// implements stub.IPlayerConnection.
type playerShardReceiver struct {
	player *Player
}

func (psr *playerShardReceiver) Init(player *Player) {
	psr.player = player
}

func (psr *playerShardReceiver) GetEntityId() EntityId {
	return psr.player.EntityId
}

func (psr *playerShardReceiver) TransmitPacket(packet []byte) {
	psr.player.TransmitPacket(packet)
}

func (psr *playerShardReceiver) ReqInventorySubscribed(block BlockXyz, invTypeId InvTypeId, slots []slot.Slot) {
	// TODO
}

func (psr *playerShardReceiver) ReqInventorySlotUpdate(block BlockXyz, slotIds []SlotId, slot slot.Slot, slotId SlotId) {
	// TODO
}

func (psr *playerShardReceiver) ReqInventoryCursorUpdate(block BlockXyz, cursor slot.Slot) {
	// TODO
}

func (psr *playerShardReceiver) ReqInventoryUnsubscribed(block BlockXyz) {
	// TODO
}

func (psr *playerShardReceiver) ReqPlaceHeldItem(target BlockXyz, wasHeld slot.Slot) {
	psr.player.Enqueue(func(_ *Player) {
		psr.player.reqPlaceHeldItem(&target, &wasHeld)
	})
}

func (psr *playerShardReceiver) ReqOfferItem(fromChunk ChunkXz, entityId EntityId, item slot.Slot) {
	psr.player.Enqueue(func(_ *Player) {
		psr.player.reqOfferItem(&fromChunk, entityId, &item)
	})
}

func (psr *playerShardReceiver) ReqGiveItem(atPosition AbsXyz, item slot.Slot) {
	psr.player.Enqueue(func(_ *Player) {
		psr.player.reqGiveItem(&atPosition, &item)
	})
}