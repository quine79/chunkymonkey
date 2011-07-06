package gamerules

import (
	"io"
	"os"

	"chunkymonkey/proto"
	. "chunkymonkey/types"
)

// Represents an inventory slot, e.g in a player's inventory, their cursor, a
// chest.
type Slot struct {
	// ItemType can be nil, specifically for empty slots.
	// TODO: change ItemType back to an ID, passing around as a pointer will
	// produce slightly odd results when passing over a network connection.
	ItemTypeId ItemTypeId
	Count    ItemCount
	Data     ItemData
}

func (s *Slot) Init() {
	s.ItemTypeId = ItemTypeIdNull
	s.Count = 0
	s.Data = 0
}

func (s *Slot) Equals(other *Slot) bool {
	return (s.ItemTypeId == other.ItemTypeId &&
		s.Count == other.Count &&
		s.Data == other.Data)
}

func (s *Slot) IsSameType(other *Slot) bool {
	return (s.ItemTypeId == other.ItemTypeId &&
		s.Data == other.Data)
}

func (s *Slot) IsCompatible(other *Slot) bool {
	return s.IsEmpty() || other.IsEmpty() || s.IsSameType(other)
}

// MaxStack returns the maximum number of items that can be held in the slot
// for its current item type. It returns 0 for unknown items or MaxStackDefault
// for empty slots.
func (s *Slot) MaxStack() ItemCount {
	if s.IsEmpty() {
		return MaxStackDefault
	}

	itemType := s.ItemType()
	if itemType == nil {
		return 0
	}

	return itemType.MaxStack
}

func (s *Slot) Normalize() {
	if s.Count == 0 || s.ItemTypeId == ItemTypeIdNull {
		s.Count = 0
		s.ItemTypeId = ItemTypeIdNull
	}
}

func (s *Slot) IsEmpty() bool {
	return s.Count == 0 || s.ItemTypeId == ItemTypeIdNull
}

func (s *Slot) ItemType() (itemType *ItemType) {
	var ok bool
	if itemType, ok = Items[s.ItemTypeId]; !ok {
		itemType = nil
	}
	return
}

func (s *Slot) Attr() (ItemTypeId, ItemCount, ItemData) {
	return s.ItemTypeId, s.Count, s.Data
}

func (s *Slot) SendUpdate(writer io.Writer, windowId WindowId, slotId SlotId) os.Error {
	return proto.WriteWindowSetSlot(writer, windowId, slotId, s.ItemTypeId, s.Count, s.Data)
}

func (s *Slot) SendEquipmentUpdate(writer io.Writer, entityId EntityId, slotId SlotId) os.Error {
	return proto.WriteEntityEquipment(writer, entityId, slotId, s.ItemTypeId, s.Data)
}

func (s *Slot) setCount(count ItemCount) {
	s.Count = count
	if s.Count == 0 {
		s.ItemTypeId = ItemTypeIdNull
		s.Data = 0
	}
}

// Adds as many items from the passed slot to the destination (subject) slot as
// possible, depending on stacking allowances and item types etc.
// Returns true if slots changed as a result.
func (s *Slot) Add(src *Slot) (changed bool) {
	// NOTE: This code assumes that 2*ItemType.MaxStack will not overflow the
	// ItemCount type.
	if src.IsEmpty() || !s.IsCompatible(src) {
		return
	}

	maxStack := src.MaxStack()

	if s.Count >= maxStack {
		return
	}

	s.ItemTypeId = src.ItemTypeId

	toTransfer := src.Count
	if s.Count+toTransfer > maxStack {
		toTransfer = maxStack - s.Count
	}
	if toTransfer != 0 {
		changed = true

		s.Data = src.Data

		s.setCount(s.Count + toTransfer)
		src.setCount(src.Count - toTransfer)
	}
	return
}

// AddWhole is similar to Add, but with the exception that if not all the items
// can be transferred, then none are transferred at all.
// Returns true if slots changed as a result.
func (s *Slot) AddWhole(src *Slot) (changed bool) {
	// NOTE: This code assumes that 2*ItemType.MaxStack will not overflow the
	// ItemCount type.
	if src.IsEmpty() || !s.IsCompatible(src) {
		return
	}

	srcItemType := src.ItemType()
	if srcItemType == nil {
		return
	}

	maxStack := srcItemType.MaxStack

	if src.Count+s.Count > maxStack {
		return
	}

	return s.Add(src)
}

// Swaps the contents of the slots.
// Returns true if slots changed as a result.
func (s *Slot) Swap(src *Slot) (changed bool) {
	if !s.IsSameType(src) {
		s.ItemTypeId ^= src.ItemTypeId
		src.ItemTypeId ^= s.ItemTypeId
		s.ItemTypeId ^= src.ItemTypeId
		changed = true
	}

	if s.Count != src.Count {
		s.Count ^= src.Count
		src.Count ^= s.Count
		s.Count ^= src.Count
		changed = true
	}

	if s.Data != src.Data {
		s.Data ^= src.Data
		src.Data ^= s.Data
		s.Data ^= src.Data
		changed = true
	}

	return
}

// Splits the contents of the subject slot (s) into half, half remaining in s,
// and half moving to src (odd amounts put the spare item into the src slot).
// If src is not empty, then this does nothing.
// Returns true if slots changed as a result.
func (s *Slot) Split(src *Slot) (changed bool) {
	if s.IsEmpty() || !src.IsEmpty() {
		return
	}

	changed = true
	src.ItemTypeId = s.ItemTypeId
	src.Data = s.Data

	count := s.Count >> 1
	odd := s.Count & 1

	src.setCount(count + odd)
	s.setCount(count)

	return
}

// Takes one item count from src and adds it to the subject s. It does nothing
// if the items in the slots are not compatible.
// Returns true if slots changed as a result.
func (s *Slot) AddOne(src *Slot) (changed bool) {
	if src.IsEmpty() {
		return
	}
	maxStack := src.MaxStack()

	if !s.IsSameType(src) && !s.IsEmpty() {
		return
	}

	if s.Count >= maxStack {
		return
	}

	changed = true
	s.setCount(s.Count + 1)
	s.ItemTypeId = src.ItemTypeId
	s.Data = src.Data
	src.setCount(src.Count - 1)

	return
}

// Decrement destroys one item count from the subject slot.
func (s *Slot) Decrement() (changed bool) {
	if s.Count <= 0 {
		return
	}

	s.setCount(s.Count - 1)
	changed = true
	return
}