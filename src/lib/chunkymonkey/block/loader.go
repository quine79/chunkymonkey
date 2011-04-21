package block

import (
    "fmt"
    "io"
    "json"
    "os"
    "strconv"
)

type aspectMakerFn func() (aspect IBlockAspect)

var aspectMakers map[string]aspectMakerFn

// Used specifically for json unmarshalling of block definitions.
type blockDef struct {
    BlockAttrs
    Aspect     string
    AspectArgs *aspectArgs
}

func newBlockDefFromBlockType(block *BlockType) (bd *blockDef, err os.Error) {
    var aspectArgs *aspectArgs
    aspectArgs, err = newAspectArgs(block.Aspect)
    if err != nil {
        return
    }
    bd = &blockDef{
        BlockAttrs: block.BlockAttrs,
        Aspect:     block.Aspect.Name(),
        AspectArgs: aspectArgs,
    }
    return
}

func (bd *blockDef) LoadBlockType() (block *BlockType, err os.Error) {
    // Create the Aspect attribute of the block.
    aspect, err := bd.loadAspect()
    if err != nil {
        return
    }
    block = &BlockType{
        BlockAttrs: bd.BlockAttrs,
        Aspect:     aspect,
    }
    return
}

func (bd *blockDef) loadAspect() (aspect IBlockAspect, err os.Error) {
    aspectMakerFn, ok := aspectMakers[bd.Aspect]
    if !ok {
        err = os.NewError(fmt.Sprintf("Unknown aspect type %q", bd.Aspect))
        return
    }
    aspect = aspectMakerFn()
    err = json.Unmarshal(bd.AspectArgs.Raw, aspect)
    return
}

// Defers parsing of AspectArgs until we know the aspect type.
type aspectArgs struct {
    Raw []byte
}

func newAspectArgs(block IBlockAspect) (a *aspectArgs, err os.Error) {
    var raw []byte
    raw, err = json.Marshal(block)
    if err != nil {
        return
    }
    a = &aspectArgs{
        Raw: raw,
    }
    return
}

func (a *aspectArgs) UnmarshalJSON(raw []byte) (err os.Error) {
    // Copy raw into a.Raw - the JSON library will destroy the content of the
    // argument after this function returns.
    a.Raw = make([]byte, len(raw))
    for i := range raw {
        a.Raw[i] = raw[i]
    }
    return
}

func (a *aspectArgs) MarshalJSON() (raw []byte, err os.Error) {
    raw = a.Raw
    return
}

func LoadBlockDefs(reader io.Reader) (blocks BlockTypeList, err os.Error) {
    blocksStr := make(map[string]blockDef)
    decoder := json.NewDecoder(reader)
    err = decoder.Decode(&blocksStr)

    // Find the max block ID so we allocate the correct amount of memory. Also
    // range check the IDs.
    maxId := 0
    for idStr := range blocksStr {
        var id int
        id, err = strconv.Atoi(idStr)
        if err != nil {
            return
        }
        if id < 0 || id > 255 {
            err = os.NewError(fmt.Sprintf(
                "Encountered block type with ID %d which is outside the range"+
                    "0 <= N <= 255",id))
            return
        }
        if id > maxId {
            maxId = id
        }
    }

    // Convert map string keys to ints.
    blocks = make(BlockTypeList, maxId+1)
    for idStr, blockDef := range blocksStr {
        var id int
        id, _ = strconv.Atoi(idStr)

        if blocks[id].defined {
            err = os.NewError(fmt.Sprintf(
                "Block ID %d defined more than once.", id))
        }

        var block *BlockType
        block, err = blockDef.LoadBlockType()
        if err != nil {
            return
        }
        block.defined = true
        blocks[id] = *block
    }

    return
}

func SaveBlockDefs(writer io.Writer, blocks BlockTypeList) (err os.Error) {
    blockDefs := make(map[string]blockDef)
    for id := range blocks {
        var blockDef *blockDef
        blockDef, err = newBlockDefFromBlockType(&blocks[id])
        if err != nil {
            return
        }
        blockDefs[fmt.Sprintf("%d", id)] = *blockDef
    }

    data, err := json.MarshalIndent(blockDefs, "", "  ")
    if err != nil {
        return
    }

    _, err = writer.Write(data)

    return
}

func init() {
    aspectMakers = map[string]aspectMakerFn{
        "Standard": makeStandardAspect,
        "Void":     makeVoidAspect,
    }
}