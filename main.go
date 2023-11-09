package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/oklog/ulid/v2"
	_ "github.com/oklog/ulid/v2"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"unicode/utf8"
)

type UserResForHTTPGet struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type User struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type responseMessage struct {
	Id string `json:"id"`
}

// ① GoプログラムからMySQLへ接続
var db *sql.DB

func init() {
	// ①-1

	mysqlUser := os.Getenv("MYSQL_USER")
	mysqlPwd := os.Getenv("MYSQL_PWD")
	mysqlHost := os.Getenv("MYSQL_HOST")
	mysqlDatabase := os.Getenv("MYSQL_DATABASE")

	connStr := fmt.Sprintf("%s:%s@%s/%s", mysqlUser, mysqlPwd, mysqlHost, mysqlDatabase)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		log.Fatalf("fail: sql.Open, %v\n", err)
	}
	// ①-3
	if err := db.Ping(); err != nil {
		log.Fatalf("fail: _db.Ping, %v\n", err)
	}

}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// ②-1 nameが空だった場合のエラー処理を施しています
		name := r.URL.Query().Get("name") // To be filled
		if name == " " {
			log.Println("fail: name is empty")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// ②-2 GETリクエストのクエリパラメータから条件を満たすデータを取得します。
		rows, err := db.Query("SELECT id, name, age FROM user WHERE name = ?", name)
		if err != nil {
			log.Printf("fail: db.Query, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// ②-3
		users := make([]UserResForHTTPGet, 0)
		for rows.Next() {
			var u UserResForHTTPGet
			if err := rows.Scan(&u.Id, &u.Name, &u.Age); err != nil {
				log.Printf("fail: rows.Scan, %v\n", err)

				if err := rows.Close(); err != nil { // 500を返して終了するが、その前にrowsのClose処理が必要
					log.Printf("fail: rows.Close(), %v\n", err)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}

		// ②-4 モジュール encoding/json を用いてレスポンス用ユーザースライスをJSONへ変換し、HTTPレスポンスボディに書き込みます。
		bytes, err := json.Marshal(users)
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	case http.MethodPost:
		//① idを採番
		var user User
		user.Id = ulid.Make().String() //GetNewULIDString()
		//② SQLにid, name, ageをinsert, 何らかのエラーにはinternal server error(500)

		json.NewDecoder(r.Body).Decode(&user)
		name := user.Name
		age := user.Age
		//nameが空、id,nameの文字列の長さが51文字以上、ageが20未満or80より上の時BadRequest400
		if name == " " {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if utf8.RuneCountInString(name) > 50 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if age < 20 || 80 < age {
			fmt.Println("error")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()

		_, newErr := tx.Exec("INSERT INTO user(id,name,age) VALUES(?,?,?)", user.Id, name, age)
		if newErr != nil {
			tx.Rollback()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Println("Insert", user)

		if err := tx.Commit(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//③ insertができたものはstatus code=200と,jsonでidを返す
		bytes, err := json.Marshal(responseMessage{
			Id: user.Id,
		})
		if err != nil {
			log.Printf("fail: json.Marshal, %v\n", err)
			return
		}
		if err == nil {
			w.WriteHeader(http.StatusOK)
			w.Write(bytes)
			return
		}

	default:
		log.Printf("fail: HTTP Method is %s\n", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

}

func main() {
	// ② /userでリクエストされたらnameパラメーターと一致する名前を持つレコードをJSON形式で返す
	http.HandleFunc("/user", handler)

	// ③ Ctrl+CでHTTPサーバー停止時にDBをクローズする
	closeDBWithSysCall()

	// 8000番ポートでリクエストを待ち受ける
	log.Println("Listening...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// ③ Ctrl+CでHTTPサーバー停止時にDBをクローズする
func closeDBWithSysCall() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-sig
		log.Printf("received syscall, %v", s)

		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
		log.Printf("success: db.Close()")
		os.Exit(0)
	}()
}

// ①idの採番
//func GetNewULIDString() string {
//	t := time.Now()
//	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)
//	id := ulid.MustNew(ulid.Timestamp(t), entropy)
//	return strings.ToLower(id.String())
//}
