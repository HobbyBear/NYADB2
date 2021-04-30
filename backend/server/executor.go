package server

import (
	"NYADB2/backend/parser"
	"NYADB2/backend/parser/statement"
	"NYADB2/backend/tbm"
	"NYADB2/backend/tm"
	"NYADB2/backend/utils"
	"errors"
)

var (
	ErrNoNestedTransaction = errors.New("No Nested Transaction.")
	ErrNotInAnyTransaction = errors.New("Not in any transaction.")
)

type Executor interface {
	Execute(sql []byte) ([]byte, error)
	Close()
}

type executor struct {
	xid tm.XID
	tbm tbm.TableManager
}

func NewExecutor(tbm tbm.TableManager) *executor {
	return &executor{
		tbm: tbm,
	}
}

func (e *executor) Close() {
	if e.xid != 0 {
		utils.Info("Abnormal Abort: ", e.xid)
		e.tbm.Abort(e.xid)
	}
}

func (e *executor) Execute(sql []byte) ([]byte, error) {
	utils.Info("Execute: ", string(sql))

	stat, err := parser.Parse(sql)
	if err != nil {
		return nil, err
	}

	var result []byte
	switch st := stat.(type) {
	case *statement.Begin:
		if e.xid != 0 {
			return nil, ErrNoNestedTransaction
		}
		e.xid, result = e.tbm.Begin(st)
		return result, nil
	case *statement.Commit:
		if e.xid == 0 {
			return nil, ErrNotInAnyTransaction
		}
		result, err = e.tbm.Commit(e.xid)
		if err != nil {
			return nil, err
		}
		e.xid = 0
		return result, nil
	case *statement.Abort:
		if e.xid == 0 {
			return nil, ErrNotInAnyTransaction
		}
		result = e.tbm.Abort(e.xid)
		e.xid = 0
		return result, nil
	default:
		return e.execute2(st)
	}
}

func (e *executor) execute2(stat interface{}) ([]byte, error) {
	var err error
	tmpTransaction := false
	if e.xid == 0 { // 创建一个临时事务
		tmpTransaction = true
		e.xid, _ = e.tbm.Begin(new(statement.Begin))
	}
	defer func() {
		if tmpTransaction == true { // 结束这个临时事务
			if err != nil {
				e.tbm.Abort(e.xid)
			} else {
				_, err = e.tbm.Commit(e.xid)
				utils.Assert(err == nil)
			}
			e.xid = 0
		}
	}()

	var result []byte
	switch st := stat.(type) {
	case *statement.Show:
		result = e.tbm.Show(e.xid)
	case *statement.Create:
		result, err = e.tbm.Create(e.xid, st)
	case *statement.Read:
		result, err = e.tbm.Read(e.xid, st)
	case *statement.Insert:
		result, err = e.tbm.Insert(e.xid, st)
	case *statement.Delete:
		result, err = e.tbm.Delete(e.xid, st)
	case *statement.Update:
		result, err = e.tbm.Update(e.xid, st)
	}

	return result, err
}
