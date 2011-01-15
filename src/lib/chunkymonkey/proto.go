package proto

import (
    "io"
    "os"
    "fmt"
    "bytes"
    "encoding/binary"
    "compress/zlib"

    .   "chunkymonkey/types"
)

const (
    // Currently only this protocol version is supported
    protocolVersion = 8

    // Packet type IDs
    packetIDKeepAlive            = 0x00
    packetIDLogin                = 0x01
    packetIDHandshake            = 0x02
    packetIDChatMessage          = 0x03
    packetIDTimeUpdate           = 0x04
    packetIDEntityEquipment      = 0x05
    packetIDSpawnPosition        = 0x06
    packetIDUseEntity            = 0x07
    packetIDUpdateHealth         = 0x08
    packetIDFlying               = 0x0a
    packetIDPlayerPosition       = 0x0b
    packetIDPlayerLook           = 0x0c
    packetIDPlayerPositionLook   = 0x0d
    packetIDPlayerDigging        = 0x0e
    packetIDPlayerBlockPlacement = 0x0f
    packetIDHoldingChange        = 0x10
    packetIDPlayerAnimation      = 0x12
    packetIDNamedEntitySpawn     = 0x14
    packetIDPickupSpawn          = 0x15
    packetIDItemCollect          = 0x16
    packetIDEntitySpawn          = 0x18
    packetIDUnknownX19           = 0x19
    packetIDEntityVelocity       = 0x1c
    packetIDEntity               = 0x1e
    packetIDEntityDestroy        = 0x1d
    packetIDEntityRelMove        = 0x1f
    packetIDEntityLook           = 0x20
    packetIDEntityLookAndRelMove = 0x21
    packetIDEntityTeleport       = 0x22
    packetIDEntityStatus         = 0x26
    packetIDUnknownX28           = 0x28
    packetIDPreChunk             = 0x32
    packetIDMapChunk             = 0x33
    packetIDBlockChangeMulti     = 0x34
    packetIDBlockChange          = 0x35
    packetIDUnknownX36           = 0x36
    packetIDWindowClick          = 0x66
    packetIDSetSlot              = 0x67
    packetIDWindowItems          = 0x68
    packetIDDisconnect           = 0xff

    // Inventory types
    // FIXME remove these with the WritePlayerInventory
    inventoryTypeMain     = -1
    inventoryTypeArmor    = -2
    inventoryTypeCrafting = -3
)

// Packets commonly received by both client and server
type RecvHandler interface {
    RecvKeepAlive()
    RecvChatMessage(message string)
    RecvOnGround(onGround bool)
    RecvPlayerPosition(position *AbsXYZ, stance AbsCoord, onGround bool)
    RecvPlayerLook(look *LookDegrees, onGround bool)
    RecvPlayerDigging(status DigStatus, blockLoc *BlockXYZ, face Face)
    RecvPlayerBlockPlacement(itemID ItemID, blockLoc *BlockXYZ, direction Face, amount ItemCount, uses ItemUses)
    RecvPlayerAnimation(animation PlayerAnimation)
    RecvDisconnect(reason string)
}

// Servers to the protocol must implement this interface to receive packets
type ServerRecvHandler interface {
    RecvHandler
    RecvHoldingChange(itemID ItemID)
    ServerRecvWindowClick(windowID WindowID, slot SlotID, rightClick bool, txID TxID, itemID ItemID, amount ItemCount, uses ItemUses)
}

// Clients to the protocol must implement this interface to receive packets
type ClientRecvHandler interface {
    RecvHandler
    ClientRecvLogin(entityID EntityID, str1 string, str2 string, mapSeed RandomSeed, dimension DimensionID)
    ClientRecvTimeUpdate(time TimeOfDay)
    ClientRecvEntityEquipment(entityID EntityID, slot SlotID, itemID ItemID, uses ItemUses)
    ClientRecvSpawnPosition(position *BlockXYZ)
    ClientRecvUseEntity(user EntityID, target EntityID, leftClick bool)
    ClientRecvUpdateHealth(health int16)
    ClientRecvPickupSpawn(entityID EntityID, itemID ItemID, count ItemCount, uses ItemUses, location *AbsIntXYZ, yaw, pitch, roll AngleBytes)
    ClientRecvItemCollect(collectedItem EntityID, collector EntityID)
    ClientRecvEntitySpawn(entityID EntityID, mobType EntityMobType, position *AbsIntXYZ, yaw AngleBytes, pitch AngleBytes, data []UnknownEntityExtra)
    ClientRecvUnknownX19(field1 int32, field2 string, field3, field4, field5, field6 int32)
    ClientRecvEntityVelocity(entityID EntityID, velocity *Velocity)
    ClientRecvEntityDestroy(entityID EntityID)
    ClientRecvEntity(entityID EntityID)
    ClientRecvEntityRelMove(entityID EntityID, movement *RelMove)
    ClientRecvEntityLook(entityID EntityID, yaw, pitch AngleBytes)
    ClientRecvEntityStatus(entityID EntityID, status EntityStatus)
    ClientRecvUnknownX28(field1 int32, data []UnknownEntityExtra)
    ClientRecvPreChunk(position *ChunkXZ, mode ChunkLoadMode)
    ClientRecvMapChunk(position *BlockXYZ, size *SubChunkSize, data []byte)
    ClientRecvBlockChangeMulti(chunkLoc *ChunkXZ, blockCoords []SubChunkXYZ, blockTypes []BlockID, blockMetaData []byte)
    ClientRecvBlockChange(blockLoc *BlockXYZ, blockType BlockID, blockMetaData byte)
    ClientRecvUnknownX36(field1 int32, field2 int16, field3 int32, field4, field5 byte)
    ClientRecvSetSlot(windowID WindowID, slot SlotID, itemID ItemID, amount ItemCount, uses ItemUses)
    ClientRecvWindowItems(windowID WindowID, items []WindowSlot)
}

// Common protocol helper functions

func boolToByte(b bool) byte {
    if b {
        return 1
    }
    return 0
}

func byteToBool(b byte) bool {
    return b != 0
}

func readString(reader io.Reader) (s string, err os.Error) {
    var length int16
    err = binary.Read(reader, binary.BigEndian, &length)
    if err != nil {
        return
    }

    bs := make([]byte, uint16(length))
    _, err = io.ReadFull(reader, bs)
    return string(bs), err
}

func WriteString(writer io.Writer, s string) (err os.Error) {
    bs := []byte(s)

    err = binary.Write(writer, binary.BigEndian, int16(len(bs)))
    if err != nil {
        return
    }

    _, err = writer.Write(bs)
    return
}

type WindowSlot struct {
    ItemID ItemID
    Amount ItemCount
    Uses   ItemUses
}

type UnknownEntityExtra struct {
    Field1 byte
    Field2 byte
    Field3 interface{}
}

// Reads extra data from the end of certain packets, whose meaning isn't known
// yet. Currently all this code does is read and discard bytes.
// TODO update to pull useful data out as it becomes understood
// http://pastebin.com/HHW52Awn
func readUnknownExtra(reader io.Reader) (data []UnknownEntityExtra, err os.Error) {
    var entryType byte

    var field1, field2 byte
    var field3 interface{}

    for {
        err = binary.Read(reader, binary.BigEndian, &entryType)
        if err != nil {
            return
        }
        if entryType == 127 {
            break
        }

        switch field1 := (entryType & 0xe0) >> 5; field1 {
        case 0:
            var byteVal byte
            err = binary.Read(reader, binary.BigEndian, &byteVal)
            field3 = byteVal
        case 1:
            var int16Val int16
            err = binary.Read(reader, binary.BigEndian, &int16Val)
            field3 = int16Val
        case 2:
            var int32Val int32
            err = binary.Read(reader, binary.BigEndian, &int32Val)
            field3 = int32Val
        case 3:
            var floatVal float32
            err = binary.Read(reader, binary.BigEndian, &floatVal)
            field3 = floatVal
        case 4:
            var stringVal string
            stringVal, err = readString(reader)
            field3 = stringVal
        case 5:
            var position struct {
                X   int16
                Y   byte
                Z   int16
            }
            err = binary.Read(reader, binary.BigEndian, &position)
            field3 = position
        }

        data = append(data, UnknownEntityExtra{field1, field2, field3})

        if err != nil {
            return
        }
    }
    return
}

// Start of packet reader/writer functions

// Naming convention:
// * Client* functions are specific to use by clients writing to a server, and
//   reading from it.
// * Server* functions are specific to use by servers writing to clients, and
//   reading from them.
// * Those without a client or server prefix are common.


// packetIDKeepAlive

func WriteKeepAlive(writer io.Writer) os.Error {
    return binary.Write(writer, binary.BigEndian, byte(packetIDKeepAlive))
}

func readKeepAlive(reader io.Reader, handler RecvHandler) (err os.Error) {
    handler.RecvKeepAlive()
    return
}

// packetIDLogin

func ServerReadLogin(reader io.Reader) (username, password string, err os.Error) {
    var packetStart struct {
        PacketID byte
        Version  int32
    }

    err = binary.Read(reader, binary.BigEndian, &packetStart)
    if err != nil {
        return
    }
    if packetStart.PacketID != packetIDLogin {
        err = os.NewError(fmt.Sprintf("serverReadLogin: invalid packet ID %#x", packetStart.PacketID))
        return
    }
    if packetStart.Version != protocolVersion {
        err = os.NewError(fmt.Sprintf("serverReadLogin: unsupported protocol version %#x", packetStart.Version))
        return
    }

    username, err = readString(reader)
    if err != nil {
        return
    }

    password, err = readString(reader)
    if err != nil {
        return
    }

    var packetEnd struct {
        MapSeed   RandomSeed
        Dimension DimensionID
    }

    err = binary.Read(reader, binary.BigEndian, &packetEnd)

    return
}

func clientReadLogin(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var entityID EntityID

    err = binary.Read(reader, binary.BigEndian, &entityID)
    if err != nil {
        return
    }

    str1, err := readString(reader)
    if err != nil {
        return
    }

    str2, err := readString(reader)
    if err != nil {
        return
    }

    var packetEnd struct {
        MapSeed   RandomSeed
        Dimension DimensionID
    }

    err = binary.Read(reader, binary.BigEndian, &packetEnd)
    if err != nil {
        return
    }

    handler.ClientRecvLogin(
        entityID,
        str1,
        str2,
        packetEnd.MapSeed,
        packetEnd.Dimension)

    return
}

func ServerWriteLogin(writer io.Writer, entityID EntityID) (err os.Error) {
    var packetStart = struct {
        PacketID byte
        EntityID EntityID
    }{
        packetIDLogin,
        entityID,
    }
    err = binary.Write(writer, binary.BigEndian, &packetStart)
    if err != nil {
        return
    }

    // TODO unknown string
    err = WriteString(writer, "")
    if err != nil {
        return
    }

    // TODO unknown string
    err = WriteString(writer, "")
    if err != nil {
        return
    }

    var packetEnd = struct {
        MapSeed   RandomSeed
        Dimension DimensionID
    }{
        // TODO proper map seed as a parameter
        0,
        // TODO proper dimension as a parameter
        0,
    }
    return binary.Write(writer, binary.BigEndian, &packetEnd)
}

// packetIDHandshake

func ServerReadHandshake(reader io.Reader) (username string, err os.Error) {
    var packetID byte
    err = binary.Read(reader, binary.BigEndian, &packetID)
    if err != nil {
        return
    }
    if packetID != packetIDHandshake {
        err = os.NewError(fmt.Sprintf("serverReadHandshake: invalid packet ID %#x", packetID))
        return
    }

    return readString(reader)
}

func ClientReadHandshake(reader io.Reader) (connectionHash string, err os.Error) {
    var packetID byte
    err = binary.Read(reader, binary.BigEndian, &packetID)
    if err != nil {
        return
    }
    if packetID != packetIDHandshake {
        err = os.NewError(fmt.Sprintf("clientReadHandshake: invalid packet ID %#x", packetID))
        return
    }

    return readString(reader)
}

func ServerWriteHandshake(writer io.Writer, reply string) (err os.Error) {
    err = binary.Write(writer, binary.BigEndian, byte(packetIDHandshake))
    if err != nil {
        return
    }

    return WriteString(writer, reply)
}

// packetIDChatMessage

func WriteChatMessage(writer io.Writer, message string) (err os.Error) {
    err = binary.Write(writer, binary.BigEndian, byte(packetIDChatMessage))
    if err != nil {
        return
    }

    err = WriteString(writer, message)
    return
}

func readChatMessage(reader io.Reader, handler RecvHandler) (err os.Error) {
    message, err := readString(reader)
    if err != nil {
        return
    }

    // TODO sanitize chat message

    handler.RecvChatMessage(message)
    return
}

// packetIDTimeUpdate

func ServerWriteTimeUpdate(writer io.Writer, time TimeOfDay) os.Error {
    var packet = struct {
        PacketID byte
        Time     TimeOfDay
    }{
        packetIDTimeUpdate,
        time,
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadTimeUpdate(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var time TimeOfDay

    err = binary.Read(reader, binary.BigEndian, &time)
    if err != nil {
        return
    }

    handler.ClientRecvTimeUpdate(time)
    return
}

// packetIDEntityEquipment

func ServerWriteEntityEquipment(writer io.Writer, entityID EntityID, slot SlotID, itemID ItemID, uses ItemUses) (err os.Error) {
    var packet = struct {
        PacketID byte
        EntityID EntityID
        Slot     SlotID
        ItemID   ItemID
        Uses     ItemUses
    }{
        packetIDEntityEquipment,
        entityID,
        slot,
        itemID,
        uses,
    }

    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadEntityEquipment(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        Slot     SlotID
        ItemID   ItemID
        Uses     ItemUses
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvEntityEquipment(
        packet.EntityID, packet.Slot, packet.ItemID, packet.Uses)

    return
}

// packetIDSpawnPosition

func WriteSpawnPosition(writer io.Writer, position *BlockXYZ) os.Error {
    var packet = struct {
        PacketID byte
        X        BlockCoord
        Y        int32
        Z        BlockCoord
    }{
        packetIDSpawnPosition,
        position.X,
        int32(position.Y),
        position.Z,
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadSpawnPosition(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        X   BlockCoord
        Y   int32
        Z   BlockCoord
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvSpawnPosition(&BlockXYZ{
        packet.X,
        BlockYCoord(packet.Y),
        packet.Z,
    })
    return
}

// packetIDUseEntity

func clientReadUseEntity(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        User      EntityID
        Target    EntityID
        LeftClick byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvUseEntity(packet.User, packet.Target, byteToBool(packet.LeftClick))

    return
}

// packetIDUpdateHealth

func clientReadUpdateHealth(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var health int16

    err = binary.Read(reader, binary.BigEndian, &health)
    if err != nil {
        return
    }

    handler.ClientRecvUpdateHealth(health)
    return
}

// packetIDFlying

func readFlying(reader io.Reader, handler RecvHandler) (err os.Error) {
    var packet struct {
        OnGround byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvOnGround(byteToBool(packet.OnGround))
    return
}

// packetIDPlayerPosition

func WritePlayerPosition(writer io.Writer, position *AbsXYZ, stance AbsCoord, onGround bool) os.Error {
    var packet = struct {
        PacketID byte
        X        AbsCoord
        Y        AbsCoord
        Stance   AbsCoord
        Z        AbsCoord
        OnGround byte
    }{
        packetIDPlayerPosition,
        position.X,
        position.Y,
        stance,
        position.Z,
        boolToByte(onGround),
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func readPlayerPosition(reader io.Reader, handler RecvHandler) (err os.Error) {
    var packet struct {
        X        AbsCoord
        Y        AbsCoord
        Stance   AbsCoord
        Z        AbsCoord
        OnGround byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvPlayerPosition(
        &AbsXYZ{
            AbsCoord(packet.X),
            AbsCoord(packet.Y),
            AbsCoord(packet.Z),
        },
        packet.Stance,
        byteToBool(packet.OnGround))
    return
}

// packetIDPlayerLook

func readPlayerLook(reader io.Reader, handler RecvHandler) (err os.Error) {
    var packet struct {
        Yaw      AngleDegrees
        Pitch    AngleDegrees
        OnGround byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvPlayerLook(
        &LookDegrees{
            packet.Yaw,
            packet.Pitch,
        },
        byteToBool(packet.OnGround))
    return
}

func clientReadPlayerPositionLook(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        X        AbsCoord
        Y        AbsCoord
        Stance   AbsCoord
        Z        AbsCoord
        Yaw      AngleDegrees
        Pitch    AngleDegrees
        OnGround byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvPlayerPosition(
        &AbsXYZ{
            packet.X,
            packet.Y,
            packet.Z,
        },
        packet.Stance,
        byteToBool(packet.OnGround))

    handler.RecvPlayerLook(
        &LookDegrees{
            packet.Yaw,
            packet.Pitch,
        },
        byteToBool(packet.OnGround))
    return
}

// packetIDPlayerPositionLook

func ServerWritePlayerPositionLook(writer io.Writer, position *AbsXYZ, look *LookDegrees, stance AbsCoord, onGround bool) os.Error {
    var packet = struct {
        PacketID byte
        X        AbsCoord
        Y        AbsCoord
        Stance   AbsCoord
        Z        AbsCoord
        Yaw      AngleDegrees
        Pitch    AngleDegrees
        OnGround byte
    }{
        packetIDPlayerPositionLook,
        position.X,
        position.Y,
        stance,
        position.Z,
        look.Yaw,
        look.Pitch,
        boolToByte(onGround),
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func serverReadPlayerPositionLook(reader io.Reader, handler ServerRecvHandler) (err os.Error) {
    var packet struct {
        X        AbsCoord
        Stance   AbsCoord
        Y        AbsCoord
        Z        AbsCoord
        Yaw      AngleDegrees
        Pitch    AngleDegrees
        OnGround byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvPlayerPosition(
        &AbsXYZ{
            packet.X,
            packet.Y,
            packet.Z,
        },
        packet.Stance,
        byteToBool(packet.OnGround))

    handler.RecvPlayerLook(
        &LookDegrees{
            packet.Yaw,
            packet.Pitch,
        },
        byteToBool(packet.OnGround))
    return
}

// packetIDPlayerDigging

func readPlayerDigging(reader io.Reader, handler RecvHandler) (err os.Error) {
    var packet struct {
        Status DigStatus
        X      BlockCoord
        Y      BlockYCoord
        Z      BlockCoord
        Face   Face
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvPlayerDigging(
        packet.Status,
        &BlockXYZ{packet.X, packet.Y, packet.Z},
        packet.Face)
    return
}

// packetIDPlayerBlockPlacement

func readPlayerBlockPlacement(reader io.Reader, handler RecvHandler) (err os.Error) {
    var packet struct {
        X      BlockCoord
        Y      BlockYCoord
        Z      BlockCoord
        Face   Face
        ItemID ItemID
    }
    var packetExtra struct {
        Amount ItemCount
        Uses   ItemUses
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    if packet.ItemID >= 0 {
        err = binary.Read(reader, binary.BigEndian, &packetExtra)
        if err != nil {
            return
        }
    }

    handler.RecvPlayerBlockPlacement(
        packet.ItemID,
        &BlockXYZ{
            packet.X,
            packet.Y,
            packet.Z,
        },
        packet.Face,
        packetExtra.Amount,
        packetExtra.Uses)
    return
}

// packetIDHoldingChange

func serverReadHoldingChange(reader io.Reader, handler ServerRecvHandler) (err os.Error) {
    var packet struct {
        ItemID ItemID
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvHoldingChange(packet.ItemID)
    return
}

// packetIDPlayerAnimation

func readPlayerAnimation(reader io.Reader, handler RecvHandler) (err os.Error) {
    var packet struct {
        EntityID  EntityID
        Animation PlayerAnimation
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.RecvPlayerAnimation(packet.Animation)
    return
}

// packetIDNamedEntitySpawn

func WriteNamedEntitySpawn(writer io.Writer, entityID EntityID, name string, position *AbsIntXYZ, look *LookBytes, currentItem ItemID) (err os.Error) {
    var packetStart = struct {
        PacketID byte
        EntityID EntityID
    }{
        packetIDNamedEntitySpawn,
        entityID,
    }

    err = binary.Write(writer, binary.BigEndian, &packetStart)
    if err != nil {
        return
    }

    err = WriteString(writer, name)
    if err != nil {
        return
    }

    var packetFinish = struct {
        X           AbsIntCoord
        Y           AbsIntCoord
        Z           AbsIntCoord
        Yaw         AngleBytes
        Pitch       AngleBytes
        CurrentItem ItemID
    }{
        position.X,
        position.Y,
        position.Z,
        look.Yaw,
        look.Pitch,
        currentItem,
    }

    err = binary.Write(writer, binary.BigEndian, &packetFinish)
    return
}

// packetIDPickupSpawn

func WritePickupSpawn(writer io.Writer, entityID EntityID, itemType ItemID, amount ItemCount, position *AbsIntXYZ, orientation *OrientationBytes) os.Error {
    var packet = struct {
        PacketID byte
        EntityID EntityID
        ItemID   ItemID
        Count    ItemCount
        // TODO check this field
        Uses  ItemUses
        X     AbsIntCoord
        Y     AbsIntCoord
        Z     AbsIntCoord
        Yaw   AngleBytes
        Pitch AngleBytes
        Roll  AngleBytes
    }{
        packetIDPickupSpawn,
        entityID,
        itemType,
        amount,
        // TODO pass proper uses value
        0,
        position.X,
        position.Y,
        position.Z,
        orientation.Yaw,
        orientation.Pitch,
        orientation.Roll,
    }

    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadPickupSpawn(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        ItemID   ItemID
        Count    ItemCount
        Uses     ItemUses
        X        AbsIntCoord
        Y        AbsIntCoord
        Z        AbsIntCoord
        Yaw      AngleBytes
        Pitch    AngleBytes
        Roll     AngleBytes
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvPickupSpawn(
        packet.EntityID,
        packet.ItemID,
        packet.Count,
        packet.Uses,
        &AbsIntXYZ{packet.X, packet.Y, packet.Z},
        packet.Yaw,
        packet.Pitch,
        packet.Roll)

    return
}

// packetIDItemCollect

func clientReadItemCollect(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        CollectedItem EntityID
        Collector     EntityID
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvItemCollect(packet.CollectedItem, packet.Collector)

    return
}

// packetIDEntitySpawn

func clientReadEntitySpawn(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        MobType  EntityMobType
        X        AbsIntCoord
        Y        AbsIntCoord
        Z        AbsIntCoord
        Yaw      AngleBytes
        Pitch    AngleBytes
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    data, err := readUnknownExtra(reader)
    if err != nil {
        return
    }

    handler.ClientRecvEntitySpawn(
        EntityID(packet.EntityID), packet.MobType,
        &AbsIntXYZ{packet.X, packet.Y, packet.Z},
        packet.Yaw, packet.Pitch, data)

    return err
}

// packetIDUnknownX19

// TODO determine what this packet is
func clientReadUnknownX19(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var Field1 int32
    err = binary.Read(reader, binary.BigEndian, &Field1)
    if err != nil {
        return
    }

    Field2, err := readString(reader)
    if err != nil {
        return
    }

    var packetEnd struct {
        Field3, Field4, Field5, Field6 int32
    }

    err = binary.Read(reader, binary.BigEndian, &packetEnd)
    if err != nil {
        return
    }

    handler.ClientRecvUnknownX19(
        Field1, Field2,
        packetEnd.Field3, packetEnd.Field4, packetEnd.Field5, packetEnd.Field6)

    return
}

// packetIDEntityVelocity

func clientReadEntityVelocity(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        X, Y, Z  VelocityComponent
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvEntityVelocity(
        packet.EntityID,
        &Velocity{packet.X, packet.Y, packet.Z})

    return
}

// packetIDEntity

func clientReadEntity(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var entityID EntityID

    err = binary.Read(reader, binary.BigEndian, &entityID)
    if err != nil {
        return
    }

    handler.ClientRecvEntity(entityID)

    return
}

// packetIDEntityDestroy

func WriteEntityDestroy(writer io.Writer, entityID EntityID) os.Error {
    var packet = struct {
        PacketID byte
        EntityID EntityID
    }{
        packetIDEntityDestroy,
        entityID,
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadEntityDestroy(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var entityID EntityID

    err = binary.Read(reader, binary.BigEndian, &entityID)
    if err != nil {
        return
    }

    handler.ClientRecvEntityDestroy(entityID)

    return
}

// packetIDEntityRelMove

func clientReadEntityRelMove(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        X, Y, Z  RelMoveCoord
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvEntityRelMove(
        packet.EntityID,
        &RelMove{packet.X, packet.Y, packet.Z})

    return
}

// packetIDEntityLook

func WriteEntityLook(writer io.Writer, entityID EntityID, look *LookBytes) os.Error {
    var packet = struct {
        PacketID byte
        EntityID EntityID
        Yaw      AngleBytes
        Pitch    AngleBytes
    }{
        packetIDEntityLook,
        entityID,
        look.Yaw,
        look.Pitch,
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadEntityLook(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        Yaw      AngleBytes
        Pitch    AngleBytes
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvEntityLook(
        packet.EntityID,
        packet.Yaw, packet.Pitch)

    return
}

// packetIDEntityLookAndRelMove

func clientReadEntityLookAndRelMove(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        X, Y, Z  RelMoveCoord
        Yaw      AngleBytes
        Pitch    AngleBytes
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvEntityRelMove(
        packet.EntityID,
        &RelMove{packet.X, packet.Y, packet.Z})

    handler.ClientRecvEntityLook(
        packet.EntityID,
        packet.Yaw, packet.Pitch)

    return
}

// packetIDEntityTeleport

func WriteEntityTeleport(writer io.Writer, entityID EntityID, position *AbsIntXYZ, look *LookBytes) os.Error {
    var packet = struct {
        PacketID byte
        EntityID EntityID
        X        AbsIntCoord
        Y        AbsIntCoord
        Z        AbsIntCoord
        Yaw      AngleBytes
        Pitch    AngleBytes
    }{
        packetIDEntityTeleport,
        entityID,
        position.X,
        position.Y,
        position.Z,
        look.Yaw,
        look.Pitch,
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

// packetIDEntityStatus

func clientReadEntityStatus(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        EntityID EntityID
        Status   EntityStatus
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvEntityStatus(packet.EntityID, packet.Status)

    return
}

// packetIDUnknownX28

func clientReadUnknownX28(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var field1 int32

    err = binary.Read(reader, binary.BigEndian, &field1)
    if err != nil {
        return
    }

    data, err := readUnknownExtra(reader)
    if err != nil {
        return
    }

    handler.ClientRecvUnknownX28(field1, data)

    return
}

// packetIDPreChunk

func WritePreChunk(writer io.Writer, chunkLoc *ChunkXZ, mode ChunkLoadMode) os.Error {
    var packet = struct {
        PacketID byte
        X        ChunkCoord
        Z        ChunkCoord
        Mode     ChunkLoadMode
    }{
        packetIDPreChunk,
        chunkLoc.X,
        chunkLoc.Z,
        mode,
    }
    return binary.Write(writer, binary.BigEndian, &packet)
}

func clientReadPreChunk(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        X    ChunkCoord
        Z    ChunkCoord
        Mode ChunkLoadMode
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvPreChunk(&ChunkXZ{packet.X, packet.Z}, packet.Mode)

    return
}

// packetIDMapChunk

func WriteMapChunk(writer io.Writer, chunkLoc *ChunkXZ, blocks, blockData, blockLight, skyLight []byte) (err os.Error) {
    buf := &bytes.Buffer{}
    compressed, err := zlib.NewWriter(buf)
    if err != nil {
        return
    }

    compressed.Write(blocks)
    compressed.Write(blockData)
    compressed.Write(blockLight)
    compressed.Write(skyLight)
    compressed.Close()
    bs := buf.Bytes()

    chunkCornerLoc := chunkLoc.GetChunkCornerBlockXY()

    var packet = struct {
        PacketID         byte
        X                BlockCoord
        Y                BlockYCoord
        Z                BlockCoord
        SizeX            SubChunkSizeCoord
        SizeY            SubChunkSizeCoord
        SizeZ            SubChunkSizeCoord
        CompressedLength int32
    }{
        packetIDMapChunk,
        chunkCornerLoc.X,
        chunkCornerLoc.Y,
        chunkCornerLoc.Z,
        ChunkSizeX - 1,
        ChunkSizeY - 1,
        ChunkSizeZ - 1,
        int32(len(bs)),
    }

    err = binary.Write(writer, binary.BigEndian, &packet)
    if err != nil {
        return
    }
    err = binary.Write(writer, binary.BigEndian, bs)
    return
}

func clientReadMapChunk(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        X                BlockCoord
        Y                BlockYCoord
        Z                BlockCoord
        SizeX            SubChunkSizeCoord
        SizeY            SubChunkSizeCoord
        SizeZ            SubChunkSizeCoord
        CompressedLength int32
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    // TODO extract block data from raw data field, and pass on to handler
    data := make([]byte, packet.CompressedLength)
    _, err = io.ReadFull(reader, data)
    if err != nil {
        return
    }

    handler.ClientRecvMapChunk(
        &BlockXYZ{packet.X, BlockYCoord(packet.Y), packet.Z},
        &SubChunkSize{packet.SizeX, packet.SizeY, packet.SizeZ},
        data)
    return
}

// packetIDBlockChangeMulti

func clientReadBlockChangeMulti(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        ChunkX ChunkCoord
        ChunkZ ChunkCoord
        Count  int16
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    rawBlockLocs := make([]int16, packet.Count)
    blockTypes := make([]BlockID, packet.Count)
    // TODO check if blockMetadata is Count bytes or nibbles long
    blockMetadata := make([]byte, packet.Count)

    err = binary.Read(reader, binary.BigEndian, rawBlockLocs)
    err = binary.Read(reader, binary.BigEndian, blockTypes)
    err = binary.Read(reader, binary.BigEndian, blockMetadata)

    blockLocs := make([]SubChunkXYZ, packet.Count)
    for rawLoc := range rawBlockLocs {
        blockLocs = append(
            blockLocs,
            SubChunkXYZ{
                X:  SubChunkCoord(rawLoc >> 12),
                Y:  SubChunkCoord(rawLoc & 0xff),
                Z:  SubChunkCoord((rawLoc >> 8) & 0xff),
            })
    }

    handler.ClientRecvBlockChangeMulti(
        &ChunkXZ{packet.ChunkX, packet.ChunkZ},
        blockLocs,
        blockTypes,
        blockMetadata)

    return
}

// packetIDBlockChange

func WriteBlockChange(writer io.Writer, blockLoc *BlockXYZ, blockType BlockID, blockMetaData byte) (err os.Error) {
    var packet = struct {
        PacketID      byte
        X             BlockCoord
        Y             BlockYCoord
        Z             BlockCoord
        BlockType     BlockID
        BlockMetadata byte
    }{
        packetIDBlockChange,
        blockLoc.X,
        blockLoc.Y,
        blockLoc.Z,
        blockType,
        blockMetaData,
    }
    err = binary.Write(writer, binary.BigEndian, &packet)
    return
}

func clientReadBlockChange(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        X             BlockCoord
        Y             BlockYCoord
        Z             BlockCoord
        BlockType     BlockID
        BlockMetadata byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)
    if err != nil {
        return
    }

    handler.ClientRecvBlockChange(
        &BlockXYZ{packet.X, packet.Y, packet.Z},
        packet.BlockType,
        packet.BlockMetadata)

    return
}

// packetIDUnknownX36

func clientReadUnknownX36(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packet struct {
        Field1         int32
        Field2         int16
        Field3         int32
        Field4, Field5 byte
    }

    err = binary.Read(reader, binary.BigEndian, &packet)

    if err != nil {
        return
    }

    handler.ClientRecvUnknownX36(packet.Field1, packet.Field2, packet.Field3, packet.Field4, packet.Field5)

    return
}

// packetIDWindowClick

func serverReadWindowClick(reader io.Reader, handler ServerRecvHandler) (err os.Error) {
    var packetStart struct {
        WindowID   WindowID
        Slot       SlotID
        RightClick byte
        TxID       TxID
        ItemID     ItemID
    }

    err = binary.Read(reader, binary.BigEndian, &packetStart)
    if err != nil {
        return
    }

    var packetEnd struct {
        Amount ItemCount
        Uses   ItemUses
    }

    if packetStart.ItemID != -1 {
        err = binary.Read(reader, binary.BigEndian, &packetEnd)
        if err != nil {
            return
        }
    }

    handler.ServerRecvWindowClick(
        packetStart.WindowID,
        packetStart.Slot,
        byteToBool(packetStart.RightClick),
        packetStart.TxID,
        packetStart.ItemID,
        packetEnd.Amount,
        packetEnd.Uses)

    return
}

// packetIDSetSlot

func clientReadSetSlot(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packetStart struct {
        WindowID WindowID
        Slot     SlotID
        ItemID   ItemID
    }

    err = binary.Read(reader, binary.BigEndian, &packetStart)
    if err != nil {
        return
    }

    var packetEnd struct {
        Amount ItemCount
        Uses   ItemUses
    }

    if packetStart.ItemID != -1 {
        err = binary.Read(reader, binary.BigEndian, &packetEnd)
        if err != nil {
            return
        }
    }

    handler.ClientRecvSetSlot(
        packetStart.WindowID,
        packetStart.Slot,
        packetStart.ItemID,
        packetEnd.Amount,
        packetEnd.Uses)

    return
}

// packetIDWindowItems

func clientReadWindowItems(reader io.Reader, handler ClientRecvHandler) (err os.Error) {
    var packetStart struct {
        WindowID WindowID
        Count    int16
    }

    err = binary.Read(reader, binary.BigEndian, &packetStart)
    if err != nil {
        return
    }

    var itemID ItemID

    items := make([]WindowSlot, packetStart.Count)

    for i := int16(0); i < packetStart.Count; i++ {
        err = binary.Read(reader, binary.BigEndian, &itemID)
        if err != nil {
            return
        }

        var itemInfo struct {
            Amount ItemCount
            Uses   ItemUses
        }
        if itemID != -1 {
            err = binary.Read(reader, binary.BigEndian, &itemInfo)
            if err != nil {
                return
            }
        }

        items = append(items, WindowSlot{
            ItemID: itemID,
            Amount: itemInfo.Amount,
            Uses:   itemInfo.Uses,
        })
    }

    handler.ClientRecvWindowItems(
        packetStart.WindowID,
        items)

    return
}

// packetIDDisconnect

func readDisconnect(reader io.Reader, handler RecvHandler) (err os.Error) {
    reason, err := readString(reader)
    if err != nil {
        return
    }

    handler.RecvDisconnect(reason)
    return
}

func WriteDisconnect(writer io.Writer, reason string) (err os.Error) {
    buf := &bytes.Buffer{}
    binary.Write(buf, binary.BigEndian, byte(packetIDDisconnect))
    WriteString(buf, reason)
    _, err = writer.Write(buf.Bytes())
    return
}


// End of packet reader/writer functions


type commonPacketHandler func(io.Reader, RecvHandler) os.Error
type csPacketHandler func(io.Reader, ServerRecvHandler) os.Error
type scPacketHandler func(io.Reader, ClientRecvHandler) os.Error

type commonPacketReaderMap map[byte]commonPacketHandler
type csPacketReaderMap map[byte]csPacketHandler
type scPacketReaderMap map[byte]scPacketHandler

// Common packet mapping
var commonReadFns = commonPacketReaderMap{
    packetIDKeepAlive:            readKeepAlive,
    packetIDChatMessage:          readChatMessage,
    packetIDFlying:               readFlying,
    packetIDPlayerPosition:       readPlayerPosition,
    packetIDPlayerLook:           readPlayerLook,
    packetIDPlayerDigging:        readPlayerDigging,
    packetIDPlayerBlockPlacement: readPlayerBlockPlacement,
    packetIDPlayerAnimation:      readPlayerAnimation,
    packetIDDisconnect:           readDisconnect,
}

// Client->server specific packet mapping
var csReadFns = csPacketReaderMap{
    packetIDPlayerPositionLook: serverReadPlayerPositionLook,
    packetIDWindowClick:        serverReadWindowClick,
    packetIDHoldingChange:      serverReadHoldingChange,
}

// Server->client specific packet mapping
var scReadFns = scPacketReaderMap{
    packetIDLogin:                clientReadLogin,
    packetIDTimeUpdate:           clientReadTimeUpdate,
    packetIDSpawnPosition:        clientReadSpawnPosition,
    packetIDUseEntity:            clientReadUseEntity,
    packetIDUpdateHealth:         clientReadUpdateHealth,
    packetIDPlayerPositionLook:   clientReadPlayerPositionLook,
    packetIDEntitySpawn:          clientReadEntitySpawn,
    packetIDPickupSpawn:          clientReadPickupSpawn,
    packetIDItemCollect:          clientReadItemCollect,
    packetIDUnknownX19:           clientReadUnknownX19,
    packetIDEntityVelocity:       clientReadEntityVelocity,
    packetIDEntityDestroy:        clientReadEntityDestroy,
    packetIDEntity:               clientReadEntity,
    packetIDEntityRelMove:        clientReadEntityRelMove,
    packetIDEntityLook:           clientReadEntityLook,
    packetIDEntityLookAndRelMove: clientReadEntityLookAndRelMove,
    packetIDEntityStatus:         clientReadEntityStatus,
    packetIDUnknownX28:           clientReadUnknownX28,
    packetIDPreChunk:             clientReadPreChunk,
    packetIDMapChunk:             clientReadMapChunk,
    packetIDBlockChangeMulti:     clientReadBlockChangeMulti,
    packetIDBlockChange:          clientReadBlockChange,
    packetIDUnknownX36:           clientReadUnknownX36,
    packetIDSetSlot:              clientReadSetSlot,
    packetIDWindowItems:          clientReadWindowItems,
}

// A server should call this to receive a single packet from a client. It will
// block until a packet was successfully handled, or there was an error.
func ServerReadPacket(reader io.Reader, handler ServerRecvHandler) os.Error {
    var packetID byte

    if err := binary.Read(reader, binary.BigEndian, &packetID); err != nil {
        return err
    }

    if commonFn, ok := commonReadFns[packetID]; ok {
        return commonFn(reader, handler)
    }

    if csFn, ok := csReadFns[packetID]; ok {
        return csFn(reader, handler)
    }

    return os.NewError(fmt.Sprintf("unhandled packet type %#x", packetID))
}

// A client should call this to receive a single packet from a client. It will
// block until a packet was successfully handled, or there was an error.
func ClientReadPacket(reader io.Reader, handler ClientRecvHandler) os.Error {
    var packetID byte

    if err := binary.Read(reader, binary.BigEndian, &packetID); err != nil {
        return err
    }

    if commonFn, ok := commonReadFns[packetID]; ok {
        return commonFn(reader, handler)
    }

    if scFn, ok := scReadFns[packetID]; ok {
        return scFn(reader, handler)
    }

    return os.NewError(fmt.Sprintf("unhandled packet type %#x", packetID))
}