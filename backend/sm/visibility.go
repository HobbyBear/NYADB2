/*
	visibility.go 实现了sm的可见性逻辑.
	这部分可见性逻辑借鉴了Postgresql, 感谢开源:)
*/
package sm

import "NYADB2/backend/tm"

// IsVersionSkip 检测是否发生了版本跳跃
func IsVersionSkip(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xmax := e.XMAX()
	if t.Level == 0 { // readCommitted 不判断版本跳跃, 直接返回false
		return false
	} else {
		return tm.IsCommited(xmax) && (xmax > t.XID || t.InSnapShot(xmax))
	}
}

// IsVisible 测试e是否对t可见.
func IsVisible(tm tm.TransactionManager, t *transaction, e *entry) bool {
	if t.Level == 0 {
		return readCommitted(tm, t, e)
	} else {
		return repeatableRead(tm, t, e)
	}
	return false
}

/*
Read Committed:
    (XMIN == Ti and                          // created by Ti itself and
     XMAX == NULL                            // not deleted now
    )
    or                                       // or
    (XMIN is commited and                    // created by a commited transaction and
     (XMAX == NULL or                        // not deleted now or
      (XMAX != Ti and XMAX is not commited)  // deleted by a uncommited transaction
    ))
*/
func readCommitted(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xid := t.XID
	xmin := e.XMIN()
	xmax := e.XMAX()

	if xmin == xid && xmax == 0 {
		return true
	}

	isCommitted := tm.IsCommited(xmin)
	if isCommitted {
		if xmax == 0 {
			return true
		}
		if xmax != xid {
			isCommitted = tm.IsCommited(xmax)
			if isCommitted == false {
				return true
			}
		}
	}
	return false
}

/*
Repeatable Read:
    (XMIN == Ti and                      // created by Ti itself and
     (XMAX == NULL or                    // not deleted now or
    ))
    or                                   // or
    (XMIN is commited and                // created by a commited treansaction and
     XMIN < XID and                      // the transaction begin before Ti and
     XMIN is not in SP(Ti) and           // the transaction commited before Ti begin and
     (XMAX == NULL or                    // not deleted now or
      (XMAX != Ti and                    // deleted by another transaction but
       (XMAX is not commited or          // the transaction is not commtied now or
        XMAX > Ti or                     // begain after Ti or
        XMAX is in SP(Ti)                // not commited when Ti begain
    ))))
*/
func repeatableRead(tm tm.TransactionManager, t *transaction, e *entry) bool {
	xid := t.XID
	xmin := e.XMIN()
	xmax := e.XMAX()

	if xmin == xid && xmax == 0 {
		return true
	}

	isCommitted := tm.IsCommited(xmin)
	if isCommitted && xmin < xid && t.InSnapShot(xmin) == false {
		if xmax == 0 {
			return true
		}
		if xmax != xid {
			isCommitted = tm.IsCommited(xmax)
			if isCommitted == false || xmax > xid || t.InSnapShot(xmax) {
				return true
			}
		}
	}
	return false
}
