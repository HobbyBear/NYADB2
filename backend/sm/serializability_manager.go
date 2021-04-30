/*
	serializability_manager.go 保证了调度的可串行化, 同时实现了MVCC.

	当事务发生ErrCannotSR错误时, SM会对该事务进行自动回滚.
*/
package sm

import (
	"NYADB2/backend/dm"
	"NYADB2/backend/sm/locktable"
	"NYADB2/backend/tm"
	"NYADB2/backend/utils"
	"NYADB2/backend/utils/cacher"
	"errors"
	"sync"
)

var (
	ErrNilEntry = errors.New("Nil Entry.")
	ErrCannotSR = errors.New("Could not serialize access due to concurrent update!")
)

type SerializabilityManager interface {
	Read(xid tm.XID, uuid utils.UUID) ([]byte, bool, error)
	Insert(xid tm.XID, data []byte) (utils.UUID, error)
	Delete(xid tm.XID, uuid utils.UUID) (bool, error)

	Begin(level int) tm.XID
	Commit(xid tm.XID) error
	Abort(xid tm.XID)
}

type serializabilityManager struct {
	TM tm.TransactionManager
	DM dm.DataManager

	ec cacher.Cacher // entry cache

	tc   map[tm.XID]*transaction // active transaction cache
	lock sync.Mutex

	lt locktable.LockTable
}

func NewSerializabilityManager(tm0 tm.TransactionManager, dm dm.DataManager) *serializabilityManager {
	sm := &serializabilityManager{
		TM: tm0,
		DM: dm,
		tc: make(map[tm.XID]*transaction),
		lt: locktable.NewLockTable(),
	}

	options := new(cacher.Options)
	options.MaxHandles = 0
	options.Get = sm.getForCacher
	options.Release = sm.releaseForCacher
	ec := cacher.NewCacher(options)
	sm.ec = ec

	sm.tc[tm.SUPER_XID] = newTransaction(tm.SUPER_XID, 0, nil)

	return sm
}

func (sm *serializabilityManager) Delete(xid tm.XID, uuid utils.UUID) (bool, error) {
	sm.lock.Lock()
	t := sm.tc[xid]
	sm.lock.Unlock()

	if t.Err != nil {
		return false, t.Err
	}

	/*
		先读取并判空, 再判断死锁.
	*/
	handle, err := sm.ec.Get(uuid)
	if err == ErrNilEntry {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	e := handle.(*entry)
	defer e.Release()

	if IsVisible(sm.TM, t, e) == false { // 如果本身对其不可见, 则直接返回
		return false, nil
	}

	ok, ch := sm.lt.Add(utils.UUID(xid), uuid)
	if ok == false {
		t.Err = ErrCannotSR
		sm.abort(xid, true) // 自动撤销
		t.AutoAbortted = true
		return false, t.Err
	}
	<-ch

	// 如果之前已经被它自身所删除, 则直接返回.
	if e.XMAX() == xid {
		return false, nil
	}

	// 获得锁后, 还得进行版本跳跃检查
	skip := IsVersionSkip(sm.TM, t, e)
	if skip == true {
		t.Err = ErrCannotSR
		sm.abort(xid, true) // 自动撤销
		t.AutoAbortted = true
		return false, t.Err
	}

	// 更新其XMAX
	e.SetXMAX(xid)
	return true, nil
}

func (sm *serializabilityManager) Insert(xid tm.XID, data []byte) (utils.UUID, error) {
	sm.lock.Lock()
	t := sm.tc[xid]
	sm.lock.Unlock()

	if t.Err != nil {
		return utils.NilUUID, t.Err
	}

	raw := WrapEntryRaw(xid, data)
	return sm.DM.Insert(xid, raw)
}

func (sm *serializabilityManager) Read(xid tm.XID, uuid utils.UUID) ([]byte, bool, error) {
	sm.lock.Lock()
	t := sm.tc[xid]
	sm.lock.Unlock()

	if t.Err != nil {
		return nil, false, t.Err
	}

	handle, err := sm.ec.Get(uuid)
	if err == ErrNilEntry {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	e := handle.(*entry)
	defer e.Release()

	if IsVisible(sm.TM, t, e) {
		return e.Data(), true, nil
	} else {
		return nil, false, nil
	}
}

func (sm *serializabilityManager) Begin(level int) tm.XID {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	xid := sm.TM.Begin()
	t := newTransaction(xid, level, sm.tc)
	sm.tc[xid] = t
	return xid
}

func (sm *serializabilityManager) Commit(xid tm.XID) error {
	sm.lock.Lock()
	t := sm.tc[xid]
	sm.lock.Unlock()

	if t.Err != nil { // 只能被撤销
		return t.Err
	}

	sm.lock.Lock()
	delete(sm.tc, xid)
	sm.lock.Unlock()

	sm.lt.Remove(utils.UUID(xid))
	sm.TM.Commit(xid)
	return nil
}

func (sm *serializabilityManager) abort(xid tm.XID, auto bool) {
	sm.lock.Lock()
	t := sm.tc[xid]
	if auto == false { // 如果自动撤销, 不完全注销该事务, 只是潜在的将其回滚; 如果是手动, 则彻底注销.
		delete(sm.tc, xid)
	}
	sm.lock.Unlock()

	if t.AutoAbortted == true { // 已经被自动撤销过了
		return
	}

	sm.lt.Remove(utils.UUID(xid))
	sm.TM.Abort(xid)
}

func (sm *serializabilityManager) Abort(xid tm.XID) {
	sm.abort(xid, false) // 手动撤销
}

func (sm *serializabilityManager) ReleaseEntry(e *entry) {
	sm.ec.Release(e.selfUUID)
}

func (sm *serializabilityManager) getForCacher(uuid utils.UUID) (interface{}, error) {
	e, ok, err := LoadEntry(sm, uuid)
	if err != nil {
		return nil, err
	}
	if ok == false { // 该entry由active事务产生, 且在恢复时已经被清除
		return nil, ErrNilEntry
	}
	return e, nil
}

func (sm *serializabilityManager) releaseForCacher(underlying interface{}) {
	e := underlying.(*entry)
	e.Remove()
}
