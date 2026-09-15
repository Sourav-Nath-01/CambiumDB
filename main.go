package main

import (
	lucario "github.com/Adarsh-Kmt/Lucario"
	bplustree "github.com/Sourav-Nath-01/CambiumDB/bplustree"
	bpm "github.com/Sourav-Nath-01/CambiumDB/bufferpoolmanager"
	server "github.com/Sourav-Nath-01/CambiumDB/server"
)

func main() {

	cache := bpm.NewLRUReplacer()
	disk, metadata, _, err := bpm.NewDirectIODiskManager("cambium.db")

	if err != nil {
		panic(err)
	}

	bufferPoolManager, err := bpm.NewSimpleBufferPoolManager(5, 4096, cache, disk)

	if err != nil {
		panic(err)
	}

	wal, err := lucario.NewWAL("./lucario.wal")
	if err != nil {
		panic(err)
	}

	btree := bplustree.NewBPlusTree(0, bufferPoolManager, metadata, wal)

	server, err := server.NewServer(":8080", btree)

	if err != nil {
		panic(err)
	}

	server.Run()
}
