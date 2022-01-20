package state

import (
	// "fmt"
	"github.com/openrelayxyz/cardinal-evm/common"
	"github.com/openrelayxyz/cardinal-evm/crypto"
	"github.com/openrelayxyz/cardinal-evm/rlp"
	"github.com/openrelayxyz/cardinal-storage"
	"github.com/openrelayxyz/cardinal-types"
	"math/big"
	"testing"
)

func testWithStateDB(fn func(tx storage.Transaction, sdb StateDB) error) error {
	return testLoadedWithStateDB([]storage.KeyValue{
		{Key: []byte("/a/Data"), Value: []byte("Something")},
		{Key: []byte("/a/Data2"), Value: []byte("Something Else")},
		{Key: []byte("a"), Value: []byte("1")},
		{Key: []byte("b"), Value: []byte("2")},
	}, fn)
}
func testLoadedWithStateDB(records []storage.KeyValue, fn func(tx storage.Transaction, sdb StateDB) error) error {
	sdb := NewMemStateDB(1, 128)
	sdb.Storage.AddBlock(
		types.HexToHash("a"),
		types.Hash{},
		1,
		big.NewInt(1),
		records,
		[][]byte{},
		[]byte("0"),
	)
	return sdb.View(types.HexToHash("a"), fn)
}

func TestNull(t *testing.T) {
	if err := testWithStateDB(func(tx storage.Transaction, sdb StateDB) error {
		address := common.HexToAddress("0x823140710bf13990e4500136726d8b55")
		sdb.CreateAccount(address)
		//value := common.FromHex("0x823140710bf13990e4500136726d8b55")
		var value types.Hash

		sdb.SetState(address, types.Hash{}, value)
		sdb.Finalise()

		if value := sdb.GetState(address, types.Hash{}); value != (types.Hash{}) {
			t.Errorf("expected empty current value, got %x", value)
		}
		if value := sdb.GetCommittedState(address, types.Hash{}); value != (types.Hash{}) {
			t.Errorf("expected empty committed value, got %x", value)
		}
		return nil
	}); err != nil {
		t.Errorf(err.Error())
	}
}

func TestPreLoadedSnapshot(t *testing.T) {
	stateobjaddr := common.BytesToAddress([]byte("aa"))
	var storageaddr types.Hash
	data1 := types.BytesToHash([]byte{42})
	data2 := types.BytesToHash([]byte{43})
	data3 := types.BytesToHash([]byte{44})

	enc, _ := rlp.EncodeToBytes(data2)
	if err := testLoadedWithStateDB([]storage.KeyValue{
		{common.FromHex("632f312f612f366566626666643066343866316532393463613964343166633938333263323231393935343833363931393239363136636139613263663835346432643733372f64"), common.FromHex("c0")},
		{common.FromHex("632f312f612f366566626666643066343866316532393463613964343166633938333263323231393935343833363931393239363136636139613263663835346432643733372f732f32393064656364393534386236326138643630333435613938383338366663383462613662633935343834303038663633363266393331363065663365353633"), enc},
	},
	func(tx storage.Transaction, sdb StateDB) error {
		if v := sdb.GetState(stateobjaddr, storageaddr); v != data2 {
			t.Errorf("wrong storage value %v, want %v", v, data2)
		}
		sdb.SetState(stateobjaddr, storageaddr, data1)
		if v := sdb.GetCommittedState(stateobjaddr, storageaddr); v != data2 {
			t.Errorf("wrong committed storage value %v, want %v", v, data2)
		}
		if v := sdb.GetState(stateobjaddr, storageaddr); v != data1 {
			t.Errorf("wrong updated storage value %v, want %v", v, data1)
		}
		storageaddrhash := crypto.Keccak256Hash(storageaddr.Bytes())
		sobj := sdb.(*stateDB).getAccount(stateobjaddr)
		if val, ok := sobj.clean[storageaddrhash]; !ok || val != data2 {
			t.Errorf("wrong clean storage value %v, want %v", val, data2)
		}
		if val, ok := sobj.dirty[storageaddrhash]; !ok || val != data1 {
			t.Errorf("wrong dirty storage value %v, want %v", val, data1)
		}
		sobj.clean[storageaddrhash] = data3
		if v := sdb.GetCommittedState(stateobjaddr, storageaddr); v != data3 {
			t.Errorf("wrong updated committed storage value %v, want %v", v, data3)
		}
		return nil
	}); err != nil {
		t.Errorf(err.Error())
	}
}
func TestSnapshot(t *testing.T) {
	stateobjaddr := common.BytesToAddress([]byte("aa"))
	var storageaddr types.Hash
	data1 := types.BytesToHash([]byte{42})
	data2 := types.BytesToHash([]byte{43})
	if err := testWithStateDB(func(tx storage.Transaction, sdb StateDB) error {
		// snapshot the genesis state
		genesis := sdb.Snapshot()

		// set initial state object value
		sdb.SetState(stateobjaddr, storageaddr, data1)
		snapshot := sdb.Snapshot()

		// set a new state object value, revert it and ensure correct content
		sdb.SetState(stateobjaddr, storageaddr, data2)
		// fmt.Println(sdb.(*stateDB).kv())
		sdb.RevertToSnapshot(snapshot)

		if v := sdb.GetState(stateobjaddr, storageaddr); v != data1 {
			t.Errorf("wrong storage value %v, want %v", v, data1)
		}
		if v := sdb.GetCommittedState(stateobjaddr, storageaddr); v != (types.Hash{}) {
			t.Errorf("wrong committed storage value %v, want %v", v, types.Hash{})
		}

		// revert up to the genesis state and ensure correct content
		sdb.RevertToSnapshot(genesis)
		if v := sdb.GetState(stateobjaddr, storageaddr); v != (types.Hash{}) {
			t.Errorf("wrong storage value %v, want %v", v, types.Hash{})
		}
		if v := sdb.GetCommittedState(stateobjaddr, storageaddr); v != (types.Hash{}) {
			t.Errorf("wrong committed storage value %v, want %v", v, types.Hash{})
		}
		return nil
	}); err != nil {
		t.Errorf(err.Error())
	}
}

func TestCopySnapshot(t *testing.T) {
	stateobjaddr := common.BytesToAddress([]byte("aa"))
	var storageaddr types.Hash
	data1 := types.BytesToHash([]byte{42})
	data2 := types.BytesToHash([]byte{43})
	if err := testWithStateDB(func(tx storage.Transaction, sdb StateDB) error {
		// snapshot the genesis state
		genesis := sdb.Snapshot()

		// set initial state object value
		sdb.SetState(stateobjaddr, storageaddr, data1)
		snapshot := sdb.Snapshot()

		// set a new state object value, revert it and ensure correct content
		sdb.SetState(stateobjaddr, storageaddr, data2)
		sdb.RevertToSnapshot(snapshot)

		sdb2 := sdb.Copy()

		// revert up to the genesis state and ensure correct content
		sdb2.RevertToSnapshot(genesis)

		// Verify that sdb2 has been reverted and sdb has not

		if v := sdb.GetState(stateobjaddr, storageaddr); v != data1 {
			t.Errorf("wrong storage value %v, want %v", v, data1)
		}
		if v := sdb.GetCommittedState(stateobjaddr, storageaddr); v != (types.Hash{}) {
			t.Errorf("wrong committed storage value %v, want %v", v, types.Hash{})
		}

		if v := sdb2.GetState(stateobjaddr, storageaddr); v != (types.Hash{}) {
			t.Errorf("wrong storage value %v, want %v", v, types.Hash{})
		}
		if v := sdb2.GetCommittedState(stateobjaddr, storageaddr); v != (types.Hash{}) {
			t.Errorf("wrong committed storage value %v, want %v", v, types.Hash{})
		}
		return nil
	}); err != nil {
		t.Errorf(err.Error())
	}
}

func TestSnapshotEmpty(t *testing.T) {
	if err := testWithStateDB(func(tx storage.Transaction, sdb StateDB) error {
		sdb.RevertToSnapshot(sdb.Snapshot())
		return nil
	}); err != nil {
		t.Errorf(err.Error())
	}
}

func TestSnapshot2(t *testing.T) {

	if err := testWithStateDB(func(tx storage.Transaction, s StateDB) error {
		sdb := s.(*stateDB)
		stateobjaddr0 := common.BytesToAddress([]byte("so0"))
		stateobjaddr1 := common.BytesToAddress([]byte("so1"))
		var storageaddr types.Hash

		data0 := types.BytesToHash([]byte{17})
		data1 := types.BytesToHash([]byte{18})

		sdb.SetState(stateobjaddr0, storageaddr, data0)
		sdb.SetState(stateobjaddr1, storageaddr, data1)

		// db, trie are already non-empty values
		so0 := sdb.getAccount(stateobjaddr0)
		so0.addBalance(big.NewInt(42))
		so0.setNonce(43)
		so0.setCode([]byte{'c', 'a', 'f', 'e'})
		so0.suicided = false
		so0.deleted = false

		sdb = sdb.Copy().(*stateDB)

		// and one with deleted == true
		so1 := sdb.getAccount(stateobjaddr1)
		so1.addBalance(big.NewInt(52))
		so1.setNonce(53)
		so1.setCode([]byte{'c', 'a', 'f', 'e', '2'})
		so1.suicided = true
		so1.deleted = true

		so1 = sdb.getAccount(stateobjaddr1)
		if !so1.deleted {
			t.Fatalf("deleted object not nil when getting")
		}

		snapshot := sdb.Snapshot()
		sdb.RevertToSnapshot(snapshot)

		so0Restored := sdb.getAccount(stateobjaddr0)
		// Update lazily-loaded values before comparing.
		so0Restored.getState(sdb.tx, sdb.chainid, storageaddr)
		so0Restored.getCode(sdb.tx, sdb.chainid)
		// non-deleted is equal (restored)
		if !so0Restored.equal(so0) {
			t.Errorf("Mismatch")
		}

		// deleted should be nil, both before and after restore of state copy
		so1Restored := sdb.getAccount(stateobjaddr1)
		if !so1Restored.deleted {
			t.Fatalf("deleted object not nil after restoring snapshot: %+v", so1Restored)
		}
		return nil
	}); err != nil {
		t.Errorf(err.Error())
	}
}
