package main

import (
	"NYADB2/backend/utils"
	"math/rand"
	"os"
	"strconv"
)

func main() {
	file, _ := os.OpenFile("./create.input", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	file.Write([]byte("create table student name string, age uint32, id uint64 (index id age)\n"))
	file.Write([]byte("exit\n"))
	file.Close()

	for i := 0; i < 40; i++ {
		genInput(i, 10000)
	}
}

func genInput(id, noTasks int) {
	file, _ := os.OpenFile("./input"+strconv.Itoa(id)+".input", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	file.Write([]byte("begin\n"))
	defer func() {
		file.Write([]byte("abort\n"))
		file.Write([]byte("exit\n"))
		file.Sync()
	}()

	for i := 0; i < noTasks; i++ {
		sql := genSQL(id*noTasks+i) + "\n"
		file.Write([]byte(sql))
	}
}

func genSQL(i int) string {
	sql := "insert into student values " + string(utils.RandBytes(50)) + " " +
		strconv.Itoa(i) + " " +
		strconv.Itoa(int(rand.Uint32()%1000000000)) + " "
	return sql
}
