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

package bind

import (
	"errors"
	"github.com/taiyuechain/taiyuechain/crypto/taiCrypto"
	"io"
	"io/ioutil"

	"github.com/taiyuechain/taiyuechain/accounts/keystore"
	"github.com/taiyuechain/taiyuechain/common"
	"github.com/taiyuechain/taiyuechain/core/types"
	"github.com/taiyuechain/taiyuechain/crypto"
)

// NewTransactor is a utility method to easily create a transaction signer from
// an encrypted json key stream and the associated passphrase.
func NewTransactor(keyin io.Reader, passphrase string) (*TransactOpts, error) {
	var taiprivate taiCrypto.TaiPrivateKey
	json, err := ioutil.ReadAll(keyin)
	if err != nil {
		return nil, err
	}
	key, err := keystore.DecryptKey(json, passphrase)
	if err != nil {
		return nil, err
	}
	taiprivate.Private = key.PrivateKey.Private
	return NewKeyedTransactor(&taiprivate), nil
}

// NewKeyedTransactor is a utility method to easily create a transaction signer
// from a single private key.
func NewKeyedTransactor(key *taiCrypto.TaiPrivateKey) *TransactOpts {
	keyAddr := crypto.PubkeyToAddress(key.Private.PublicKey)
	return &TransactOpts{
		From: keyAddr,
		Signer: func(signer types.Signer, address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != keyAddr {
				return nil, errors.New("not authorized to sign this account")
			}
			signature, err := crypto.Sign(signer.Hash(tx).Bytes(), &key.Private)
			if err != nil {
				return nil, err
			}
			return tx.WithSignature(signer, signature)
		},
	}
}
