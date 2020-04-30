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

package light

import (
	"bytes"
	"context"
	"github.com/taiyuechain/taiyuechain/crypto/taiCrypto"

	"github.com/taiyuechain/taiyuechain/common"
	//"github.com/taiyuechain/taiyuechain/crypto"
	"github.com/taiyuechain/taiyuechain/core"
	"github.com/taiyuechain/taiyuechain/core/rawdb"
	"github.com/taiyuechain/taiyuechain/core/types"
	"github.com/taiyuechain/taiyuechain/rlp"
)

var thash taiCrypto.THash

//var sha3_nil = crypto.Keccak256Hash(nil)
var sha3_nil = thash.Keccak256Hash(nil)

func GetHeaderByNumber(ctx context.Context, odr OdrBackend, number uint64) (*types.Header, error) {
	db := odr.Database()
	hash := rawdb.ReadCanonicalHash(db, number)
	if (hash != common.Hash{}) {
		// if there is a canonical hash, there is a header too
		header := rawdb.ReadHeader(db, hash, number)
		if header == nil {
			panic("Canonical hash present but header not found")
		}
		return header, nil
	}

	var (
		chtCount, sectionHeadNum uint64
		sectionHead              common.Hash
	)
	if odr.ChtIndexer() != nil {
		chtCount, sectionHeadNum, sectionHead = odr.ChtIndexer().Sections()
		canonicalHash := rawdb.ReadCanonicalHash(db, sectionHeadNum)
		// if the CHT was injected as a trusted checkpoint, we have no canonical hash yet so we accept zero hash too
		for chtCount > 0 && canonicalHash != sectionHead && canonicalHash != (common.Hash{}) {
			chtCount--
			if chtCount > 0 {
				sectionHeadNum = chtCount*CHTFrequencyClient - 1
				sectionHead = odr.ChtIndexer().SectionHead(chtCount - 1)
				canonicalHash = rawdb.ReadCanonicalHash(db, sectionHeadNum)
			}
		}
	}
	if number >= chtCount*CHTFrequencyClient {
		return nil, ErrNoTrustedCht
	}
	r := &ChtRequest{ChtRoot: GetChtRoot(db, chtCount-1, sectionHead), ChtNum: chtCount - 1, BlockNum: number}
	if err := odr.Retrieve(ctx, r); err != nil {
		return nil, err
	}
	return nil, nil
}

func GetCanonicalHash(ctx context.Context, odr OdrBackend, number uint64) (common.Hash, error) {
	hash := rawdb.ReadCanonicalHash(odr.Database(), number)
	if (hash != common.Hash{}) {
		return hash, nil
	}
	header, err := GetHeaderByNumber(ctx, odr, number)
	if header != nil {
		return header.Hash(), nil
	}
	return common.Hash{}, err
}

// GetBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func GetBodyRLP(ctx context.Context, odr OdrBackend, hash common.Hash, number uint64) (rlp.RawValue, error) {
	if data := rawdb.ReadBodyRLP(odr.Database(), hash, number); data != nil {
		return data, nil
	}
	r := &BlockRequest{Hash: hash, Number: number}
	if err := odr.Retrieve(ctx, r); err != nil {
		return nil, err
	} else {
		return r.Rlp, nil
	}
}

// GetBody retrieves the block body (transactons, uncles) corresponding to the
// hash.
func GetBody(ctx context.Context, odr OdrBackend, hash common.Hash, number uint64) (*types.Body, error) {
	data, err := GetBodyRLP(ctx, odr, hash, number)
	if err != nil {
		return nil, err
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		return nil, err
	}
	return body, nil
}

// GetBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body.
func GetBlock(ctx context.Context, odr OdrBackend, hash common.Hash, number uint64) (*types.Block, error) {
	// Retrieve the block header and body contents
	header := rawdb.ReadHeader(odr.Database(), hash, number)
	if header == nil {
		return nil, ErrNoHeader
	}
	body, err := GetBody(ctx, odr, hash, number)
	if err != nil {
		return nil, err
	}
	// Reassemble the block and return
	return types.NewBlockWithHeader(header).WithBody(body.Transactions, body.Signs, body.Infos), nil
}

// GetBlockReceipts retrieves the receipts generated by the transactions included
// in a block given by its hash.
func GetBlockReceipts(ctx context.Context, odr OdrBackend, hash common.Hash, number uint64) (types.Receipts, error) {
	// Retrieve the potentially incomplete receipts from disk or network
	receipts := rawdb.ReadReceipts(odr.Database(), hash, number)
	if receipts == nil {
		r := &ReceiptsRequest{Hash: hash, Number: number}
		if err := odr.Retrieve(ctx, r); err != nil {
			return nil, err
		}
		receipts = r.Receipts
	}
	// If the receipts are incomplete, fill the derived fields
	if len(receipts) > 0 && receipts[0].TxHash == (common.Hash{}) {
		block, err := GetBlock(ctx, odr, hash, number)
		if err != nil {
			return nil, err
		}
		genesis := rawdb.ReadCanonicalHash(odr.Database(), 0)
		config := rawdb.ReadChainConfig(odr.Database(), genesis)

		if err := core.SetReceiptsData(config, block, receipts); err != nil {
			return nil, err
		}
		rawdb.WriteReceipts(odr.Database(), hash, number, receipts)
	}
	return receipts, nil
}

// GetBlockLogs retrieves the logs generated by the transactions included in a
// block given by its hash.
func GetBlockLogs(ctx context.Context, odr OdrBackend, hash common.Hash, number uint64) ([][]*types.Log, error) {
	// Retrieve the potentially incomplete receipts from disk or network
	receipts := rawdb.ReadReceipts(odr.Database(), hash, number)
	if receipts == nil {
		r := &ReceiptsRequest{Hash: hash, Number: number}
		if err := odr.Retrieve(ctx, r); err != nil {
			return nil, err
		}
		receipts = r.Receipts
	}
	// Return the logs without deriving any computed fields on the receipts
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

// GetBloomBits retrieves a batch of compressed bloomBits vectors belonging to the given bit index and section indexes
func GetBloomBits(ctx context.Context, odr OdrBackend, bitIdx uint, sectionIdxList []uint64) ([][]byte, error) {
	db := odr.Database()
	result := make([][]byte, len(sectionIdxList))
	var (
		reqList []uint64
		reqIdx  []int
	)

	var (
		bloomTrieCount, sectionHeadNum uint64
		sectionHead                    common.Hash
	)
	if odr.BloomTrieIndexer() != nil {
		bloomTrieCount, sectionHeadNum, sectionHead = odr.BloomTrieIndexer().Sections()
		canonicalHash := rawdb.ReadCanonicalHash(db, sectionHeadNum)
		// if the BloomTrie was injected as a trusted checkpoint, we have no canonical hash yet so we accept zero hash too
		for bloomTrieCount > 0 && canonicalHash != sectionHead && canonicalHash != (common.Hash{}) {
			bloomTrieCount--
			if bloomTrieCount > 0 {
				sectionHeadNum = bloomTrieCount*BloomTrieFrequency - 1
				sectionHead = odr.BloomTrieIndexer().SectionHead(bloomTrieCount - 1)
				canonicalHash = rawdb.ReadCanonicalHash(db, sectionHeadNum)
			}
		}
	}

	for i, sectionIdx := range sectionIdxList {
		sectionHead := rawdb.ReadCanonicalHash(db, (sectionIdx+1)*BloomTrieFrequency-1)
		// if we don't have the canonical hash stored for this section head number, we'll still look for
		// an entry with a zero sectionHead (we store it with zero section head too if we don't know it
		// at the time of the retrieval)
		bloomBits, err := rawdb.ReadBloomBits(db, bitIdx, sectionIdx, sectionHead)
		if err == nil {
			result[i] = bloomBits
		} else {
			if sectionIdx >= bloomTrieCount {
				return nil, ErrNoTrustedBloomTrie
			}
			reqList = append(reqList, sectionIdx)
			reqIdx = append(reqIdx, i)
		}
	}
	if reqList == nil {
		return result, nil
	}

	r := &BloomRequest{BloomTrieRoot: GetBloomTrieRoot(db, bloomTrieCount-1, sectionHead), BloomTrieNum: bloomTrieCount - 1, BitIdx: bitIdx, SectionIdxList: reqList}
	if err := odr.Retrieve(ctx, r); err != nil {
		return nil, err
	} else {
		for i, idx := range reqIdx {
			result[idx] = r.BloomBits[i]
		}
		return result, nil
	}
}
