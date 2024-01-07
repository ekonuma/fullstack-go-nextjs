package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

type User struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Email string `json:"email"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func websocketHandler(w http.ResponseWriter, r *http.Request){
	conn, err := upgrader.Upgrade(w,r,nil)

	if err != nil{
		log.Fatalln(err)
	}

	defer conn.Close()

	for{
		_, message, err := conn.ReadMessage() 

		if err != nil{
			log.Fatalln(err)
			break
		}

		log.Printf("Received message: %s", message)

		err = conn.WriteMessage(websocket.TextMessage, message)

		if err != nil{
			log.Fatalln(err)
			break
		}
	}
}

func main(){
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT, email TEXT)")
	if err != nil{
		log.Fatal(err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/go/users", getUsers(db)).Methods("GET")
	router.HandleFunc("/api/go/users", createUser(db)).Methods("POST")
	router.HandleFunc("/api/go/users/{id}", getUser(db)).Methods("GET")
	router.HandleFunc("/api/go/users/{id}", updateUser(db)).Methods("PUT")
	router.HandleFunc("/api/go/users/{id}", deleteUser(db)).Methods("DELETE")
	router.HandleFunc("/websocket", websocketHandler)


	enhanceRouter := enableCORS(jsonContentTypeMiddleware(router))
	log.Fatal(http.ListenAndServe(":8000", enhanceRouter))
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS"{
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func getUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT * FROM users")
		if err != nil{
			log.Fatal(err)
		}
		defer rows.Close();

		users := []User{}
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.Id, &u.Name, &u.Email); err != nil{
				log.Fatal(err)
			}
			users = append(users, u)	
		}
		if err := rows.Err(); err != nil {
			log.Fatal(err)
		}

		json.NewEncoder(w).Encode(users)
	}
}

func getUser(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			id := vars["id"]

			var u User

			err := db.QueryRow("SELECT * FROM users WHERE id = $1", id).Scan(&u.Id, &u.Name, &u.Email)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(u)
		}
}

func createUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u User
		json.NewDecoder(r.Body).Decode(&u)

		err := db.QueryRow("INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id", u.Name, u.Email).Scan(&u.Id)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(u)
	}
}

func updateUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u User
		json.NewDecoder(r.Body).Decode(&u)

		vars := mux.Vars(r)
		id := vars["id"]

		_, err := db.Exec("UPDATE users SET name = $1, email = $2 WHERE id = $3", u.Name, u.Email, id)
		if err != nil {
			log.Fatal(err)
		}

		var updatedUser User
		err = db.QueryRow("SELECT id, name, email FROM users WHERE id = $1", id).Scan(&updatedUser.Id, &updatedUser.Name, &updatedUser.Email)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(updatedUser)
	}

}
func deleteUser(db *sql.DB) http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]
		
		var u User
		err := db.QueryRow("SELECT * FROM users WHERE id = $1", id).Scan(&u.Id, &u.Name, &u.Email)

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			log.Fatal(err)
		}

		_, err = db.Exec("DELETE FROM users WHERE id = $1", id)
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(w).Encode(u)
	}
}