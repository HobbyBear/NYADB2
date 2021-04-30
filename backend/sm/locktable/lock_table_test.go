package locktable_test

import (
	"NYADB2/backend/sm/locktable"
	"NYADB2/backend/utils"
	"testing"
)

func TestLockTableMulti(t *testing.T) {
	lt := locktable.NewLockTable()
	_, ch1 := lt.Add(1, 1)
	_, ch2 := lt.Add(1, 2)
	_, ch3 := lt.Add(1, 3)
	_, ch4 := lt.Add(1, 4)

	<-ch1
	<-ch2
	<-ch3
	<-ch4
}

func TestLockTable(t *testing.T) {
	lt := locktable.NewLockTable()
	ok, _ := lt.Add(1, 1)
	if ok == false {
		t.Fatal("Error")
	}
	ok, _ = lt.Add(2, 2)
	if ok == false {
		t.Fatal("Error")
	}
	ok, _ = lt.Add(2, 1)
	if ok == false {
		t.Fatal("Error")
	}
	ok, _ = lt.Add(1, 2)
	if ok == true {
		t.Fatal("Error")
	}
}

func TestLockTable2(t *testing.T) {
	lt := locktable.NewLockTable()
	for i := 1; i <= 100; i++ {
		ok, ch := lt.Add(utils.UUID(i), utils.UUID(i))
		if ok == false {
			t.Fatal("Error")
		}
		go func() {
			<-ch
		}()
	}
	for i := 1; i <= 99; i++ {
		ok, ch := lt.Add(utils.UUID(i), utils.UUID(i+1))
		if ok == false {
			t.Fatal("Error")
		}
		go func() {
			<-ch
		}()
	}

	ok, _ := lt.Add(100, 1)
	if ok == true {
		t.Fatal("Error")
	}

	lt.Remove(23)
	ok, _ = lt.Add(100, 1)
	if ok == false {
		t.Fatal("Error")
	}
}
