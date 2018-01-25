package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/boltdb/bolt"
)

const dbFileName = "bc.db"
const blocksBucketName = "blocks"

// Blockchain implement interactions with a DB
type Blockchain struct {
	db *bolt.DB
}

// BlockchainIterator is used to iterate over blockchain blocks
type BlockchainIterator struct {
	currentHash []byte
	bc          *Blockchain
}

func (i *BlockchainIterator) next() *Block {
	var block *Block

	err := i.bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucketName))
		encodedBlock := b.Get(i.currentHash)
		block = deserializeBlock(encodedBlock)

		return nil
	})

	if err != nil {
		Error.Panic(err)
	}

	i.currentHash = block.Header.PrevBlockHash

	return block
}

func (bc *Blockchain) iterator() *BlockchainIterator {
	var lastHash = bc.getTopBlockHash()
	bci := &BlockchainIterator{lastHash, bc}
	return bci
}

func (bc *Blockchain) getTopBlockHash() []byte {
	var lastHash []byte
	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucketName))
		lastHash = b.Get([]byte("l"))
		return nil
	})

	if err != nil {
		Error.Panic(err)
	}

	return lastHash
}

func (bc *Blockchain) String() string {
	bci := bc.iterator()
	var strBlockchain string

	for {
		block := bci.next()
		strBlock := fmt.Sprintf("%v", block)
		strBlockchain += "[" + strconv.Itoa(block.Header.Height) + "]  "
		strBlockchain += strBlock
		strBlockchain += "\n"

		if block.isGenesisBlock() {
			break
		}
	}

	return strBlockchain
}

func (bc *Blockchain) addBlock(block *Block) {
	pow := newProofOfWork(block)

	if !pow.validate() {
		nonce, hash := pow.run()
		block.Header.Nonce = nonce
		block.Header.Hash = hash[:]
	}

	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucketName))

		if bc.isEmpty() {
			bc.putBlock(b, block.Header.Hash, block.serialize())
		} else {
			lastHash := b.Get([]byte("l"))
			encodedLastBlock := b.Get(lastHash)
			lastBlock := deserializeBlock(encodedLastBlock)

			if block.Header.Height > lastBlock.Header.Height && bytes.Compare(block.Header.PrevBlockHash, lastBlock.Header.Hash) == 0 {
				bc.putBlock(b, block.Header.Hash, block.serialize())
			} else {
				Error.Printf("Block invalid. Failed to add block. : \n%v\n", block)

			}
		}

		return nil
	})

	if err != nil {
		Error.Panic(err)
	}
}

func (bc *Blockchain) putBlock(b *bolt.Bucket, blockHash, blockData []byte) {
	err := b.Put(blockHash, blockData)
	if err != nil {
		Error.Panic(err)
	}

	err = b.Put([]byte("l"), blockHash)
	if err != nil {
		Error.Panic(err)
	}
}

func createEmptyBlockchain() *Blockchain {
	if isDbExists(dbFileName) {
		fmt.Println("Blockchain already exists.")
		return nil
	}

	db, err := bolt.Open(dbFileName, 0600, nil)
	if err != nil {
		Error.Fatal(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket([]byte(blocksBucketName))
		if err != nil {
			Error.Panic(err)
		}

		return nil
	})

	if err != nil {
		Error.Fatal(err)
	}

	bc := &Blockchain{db}
	return bc
}

func (bc *Blockchain) isEmpty() bool {
	return len(bc.getTopBlockHash()) == 0
}

// GetBestHeight returns the height of the latest block
func (bc *Blockchain) getBestHeight() int {
	var lastBlock *Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucketName))
		lastHash := b.Get([]byte("l"))
		if lastHash == nil {
			return nil
		}

		blockData := b.Get(lastHash)
		lastBlock = deserializeBlock(blockData)

		return nil
	})

	if err != nil {
		Error.Panic(err)
		return 0
	}

	if lastBlock == nil {
		return 0
	}

	return lastBlock.Header.Height
}

func (bc *Blockchain) getHashList() [][]byte {
	var hashList [][]byte
	bci := bc.iterator()

	for {
		block := bci.next()

		hashList = append(hashList, block.Header.Hash)

		if block.isGenesisBlock() {
			break
		}
	}
	return hashList
}

func isDbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

func (bc *Blockchain) getBlockByHeight(height int) *Block {
	bci := bc.iterator()

	for {
		block := bci.next()

		if block.Header.Height == height {
			return block
		}

		if block.isGenesisBlock() {
			break
		}
	}

	return nil
}

func getLocalBc() *Blockchain {
	if !isDbExists(dbFileName) {
		Info.Printf("Local blockchain not exists")
		return nil
	}

	db, err := bolt.Open(dbFileName, 0600, nil)
	if err != nil {
		Error.Fatal(err)
	}

	bc := &Blockchain{db}
	return bc
}
