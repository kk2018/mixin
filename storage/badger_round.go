package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) ReadLink(from, to crypto.Hash) (uint64, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readLink(txn, from, to)
}

func (s *BadgerStore) ReadRound(hash crypto.Hash) (*common.Round, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readRound(txn, hash)
}

func (s *BadgerStore) StartNewRound(node crypto.Hash, number uint64, references *common.RoundLink, finalStart uint64) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert only, remove in future
	if config.Debug {
		self, err := readRound(txn, node)
		if err != nil {
			return err
		}
		external, err := readRound(txn, references.External)
		if err != nil {
			return err
		}
		if self == nil || self.Number != number-1 {
			panic("self final assert error")
		}
		if external == nil {
			panic("external final not exist")
		}
		old, err := readRound(txn, references.Self)
		if err != nil {
			return err
		}
		if old != nil {
			panic("self final already exist")
		}
		link, err := readLink(txn, node, external.NodeId)
		if err != nil {
			return err
		}
		if link > external.Number {
			panic("external link backward")
		}
	}
	// assert end

	err := startNewRound(txn, node, number, references, finalStart)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func startNewRound(txn *badger.Txn, node crypto.Hash, number uint64, references *common.RoundLink, finalStart uint64) error {
	if references != nil {
		self, err := readRound(txn, node)
		if err != nil {
			return err
		}
		external, err := readRound(txn, references.External)
		if err != nil {
			return err
		}

		err = writeLink(txn, node, external.NodeId, external.Number)
		if err != nil {
			return err
		}
		self.Timestamp = finalStart
		err = writeRound(txn, references.Self, self)
		if err != nil {
			return err
		}
	}

	return writeRound(txn, node, &common.Round{
		NodeId:     node,
		Number:     number,
		References: references,
	})
}

func readLink(txn *badger.Txn, from, to crypto.Hash) (uint64, error) {
	key := graphLinkKey(from, to)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(ival), nil
}

func writeLink(txn *badger.Txn, from, to crypto.Hash, link uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, link)
	key := graphLinkKey(from, to)
	return txn.Set(key, buf)
}

func readRound(txn *badger.Txn, hash crypto.Hash) (*common.Round, error) {
	var out common.Round
	key := graphRoundKey(hash)
	err := graphReadValue(txn, key, &out)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &out, err
}

func writeRound(txn *badger.Txn, hash crypto.Hash, round *common.Round) error {
	key := graphRoundKey(hash)
	val := common.MsgpackMarshalPanic(round)
	return txn.Set(key, val)
}

func graphRoundKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixRound), hash[:]...)
}

func graphLinkKey(from, to crypto.Hash) []byte {
	link := crypto.NewHash(append(from[:], to[:]...))
	return append([]byte(graphPrefixLink), link[:]...)
}
